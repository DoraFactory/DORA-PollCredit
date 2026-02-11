package worker

import (
	"context"
	"log"
	"strings"
	"time"

	"DORAPollCredit/internal/chain"
	"DORAPollCredit/internal/models"
	"DORAPollCredit/internal/payments"
	"DORAPollCredit/internal/store"
)

type Worker struct {
	Store               *store.Store
	Chain               chain.Client
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
				for _, t := range payments.ExtractTransfers(tx.Events, order.Denom) {
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
	status, updated, err := payments.ApplyPayment(ctx, w.Store, order, tx, amount, sender)
	if err != nil {
		return err
	}
	if updated {
		log.Printf("order %s -> %s tx=%s amount=%s", order.OrderID, status, tx.Hash, amount)
	}
	return nil
}

func buildRecipientQuery(key, addr string) string {
	return key + "='" + addr + "'"
}
