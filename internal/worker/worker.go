package worker

import (
	"context"
	"errors"
	"log"
	"math/big"
	"strings"
	"time"

	"DORAPollCredit/internal/chain"
	"DORAPollCredit/internal/models"
	"DORAPollCredit/internal/pricing"
	"DORAPollCredit/internal/store"
)

type Worker struct {
	Store               *store.Store
	Chain               chain.Client
	Pricing             pricing.Service
	Denom               string
	Decimals            int
	ConfirmDepth        int64
	StartHeight         int64
	RewindBlocks        int64
	MaxBlocksPerTick    int64
	PerPage             int
	Interval            time.Duration
	WSEndpoints         []string
	WSBackfillBlocks    int64
	WSFailoverThreshold int
}

func (w *Worker) Run(ctx context.Context) {
	go w.RunWS(ctx)
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()

	for {
		if err := w.SyncOnce(ctx); err != nil {
			log.Printf("sync error: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) SyncOnce(ctx context.Context) error {
	latest, err := w.Chain.LatestHeight(ctx)
	if err != nil {
		return err
	}
	to := latest - w.ConfirmDepth
	if to <= 0 {
		return nil
	}

	last, err := w.Store.GetSyncHeight(ctx)
	if err != nil {
		return err
	}
	from := last + 1
	if last == 0 {
		if w.StartHeight > 0 {
			from = w.StartHeight
		} else {
			from = 1
		}
	} else if w.RewindBlocks > 0 {
		if last > w.RewindBlocks {
			from = last - w.RewindBlocks + 1
		} else {
			from = 1
		}
	}
	if from > to {
		return nil
	}
	if w.MaxBlocksPerTick > 0 {
		limitTo := from + w.MaxBlocksPerTick - 1
		if limitTo < to {
			to = limitTo
		}
	}

	if err := w.Store.MarkExpired(ctx, time.Now().UTC()); err != nil {
		return err
	}

	if err := w.scanRange(ctx, from, to); err != nil {
		return err
	}

	return w.Store.SetSyncHeight(ctx, to)
}

func (w *Worker) BackfillRecent(ctx context.Context, blocks int64) {
	if blocks <= 0 {
		return
	}
	latest, err := w.Chain.LatestHeight(ctx)
	if err != nil {
		log.Printf("ws backfill latest height failed: %v", err)
		return
	}
	to := latest - w.ConfirmDepth
	if to <= 0 {
		return
	}
	from := to - blocks + 1
	if from < 1 {
		from = 1
	}
	log.Printf("ws backfill range=%d..%d", from, to)
	if err := w.scanRange(ctx, from, to); err != nil {
		log.Printf("ws backfill failed: %v", err)
	}
}

func (w *Worker) scanRange(ctx context.Context, from, to int64) error {
	orders, err := w.Store.ListPendingOrders(ctx)
	if err != nil {
		return err
	}
	if len(orders) == 0 {
		log.Printf("sync range=%d..%d pending=0", from, to)
		return nil
	}
	ids := make([]string, 0, len(orders))
	for _, order := range orders {
		ids = append(ids, order.OrderID)
	}
	log.Printf("sync range=%d..%d pending=%d ids=%s", from, to, len(orders), strings.Join(ids, ","))
	for _, order := range orders {
		if err := w.scanOrder(ctx, order, from, to); err != nil {
			log.Printf("scan order %s failed: %v", order.OrderID, err)
		}
	}
	return nil
}

func (w *Worker) scanOrder(ctx context.Context, order *models.Order, from, to int64) error {
	for _, key := range []string{"transfer.recipient", "coin_received.receiver"} {
		query := buildRecipientQuery(key, order.RecipientAddress)
		page := 1
		perPage := w.PerPage
		if perPage <= 0 {
			perPage = 30
		}

		for {
			res, err := w.Chain.TxSearch(ctx, query, page, perPage)
			if err != nil {
				return err
			}
			if res.TotalCount == 0 {
				break
			}
			for _, tx := range res.Txs {
				if tx.Height < from || tx.Height > to {
					continue
				}
				if tx.Code != 0 {
					continue
				}
				for _, t := range extractTransfers(tx.Events, order.Denom) {
					if t.Recipient != order.RecipientAddress {
						continue
					}
					if err := w.applyPayment(ctx, order, tx, t.Amount, t.Sender); err != nil {
						log.Printf("apply payment failed order=%s tx=%s: %v", order.OrderID, tx.Hash, err)
					}
				}
			}

			if int64(page*perPage) >= res.TotalCount {
				break
			}
			page++
		}
	}
	return nil
}

func (w *Worker) applyPayment(ctx context.Context, order *models.Order, tx chain.Tx, amount string, sender string) error {
	paidAt := tx.Timestamp
	if paidAt.IsZero() {
		paidAt = time.Now().UTC()
	}

	cmp := compareAmount(amount, order.AmountPeaka)
	status := models.OrderPaid
	var creditIssued *int64

	switch {
	case cmp < 0:
		status = models.OrderUnderpaid
	case cmp > 0:
		status = models.OrderOverpaid
	default:
		if paidAt.After(order.ExpiresAt) {
			status = models.OrderPaidLateReprice
			credit, err := w.calcLateCredit(amount)
			if err != nil {
				return err
			}
			creditIssued = &credit
		} else {
			status = models.OrderPaid
			creditIssued = &order.CreditRequested
		}
	}

	payment := &models.Payment{
		TxHash:      tx.Hash,
		OrderID:     order.OrderID,
		FromAddress: sender,
		ToAddress:   order.RecipientAddress,
		AmountPeaka: amount,
		Denom:       order.Denom,
		Height:      tx.Height,
		BlockTime:   paidAt,
	}
	if err := w.Store.InsertPayment(ctx, payment); err != nil {
		return err
	}

	updated, err := w.Store.UpdateOrderPayment(ctx, order.OrderID, status, paidAt, tx.Hash, creditIssued)
	if err != nil {
		return err
	}
	if updated > 0 {
		log.Printf("order %s -> %s tx=%s amount=%s", order.OrderID, status, tx.Hash, amount)
	}
	return nil
}

func (w *Worker) calcLateCredit(paidPeaka string) (int64, error) {
	snap, err := w.Pricing.CurrentSnapshot(context.Background())
	if err != nil {
		return 0, err
	}

	paid, ok := new(big.Int).SetString(paidPeaka, 10)
	if !ok {
		return 0, errors.New("invalid paid amount")
	}

	pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(w.Decimals)), nil)
	num := new(big.Int).Mul(paid, big.NewInt(snap.CreditPerDora))
	quot := new(big.Int).Quo(num, pow)
	if !quot.IsInt64() {
		return 0, errors.New("credit overflow")
	}
	return quot.Int64(), nil
}

func buildRecipientQuery(key, addr string) string {
	return key + "='" + addr + "'"
}

func parseAmountForDenom(amount string, denom string) (string, bool) {
	for _, coin := range strings.Split(amount, ",") {
		coin = strings.TrimSpace(coin)
		if coin == "" {
			continue
		}
		idx := firstNonDigit(coin)
		if idx <= 0 {
			continue
		}
		amt := coin[:idx]
		den := coin[idx:]
		if den == denom {
			return amt, true
		}
	}
	return "", false
}

func firstNonDigit(s string) int {
	for i, r := range s {
		if r < '0' || r > '9' {
			return i
		}
	}
	return -1
}

func compareAmount(a, b string) int {
	ai, ok1 := new(big.Int).SetString(a, 10)
	bi, ok2 := new(big.Int).SetString(b, 10)
	if !ok1 || !ok2 {
		return 0
	}
	return ai.Cmp(bi)
}
