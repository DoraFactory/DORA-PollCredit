package payments

import (
	"context"
	"math/big"
	"strings"
	"time"

	"DORAPollCredit/internal/chain"
	"DORAPollCredit/internal/models"
	"DORAPollCredit/internal/store"
)

type Transfer struct {
	Recipient string
	Amount    string
	Sender    string
}

func ExtractTransfers(events []chain.Event, denom string) []Transfer {
	var out []Transfer
	for _, ev := range events {
		switch ev.Type {
		case "transfer":
			var amt string
			var rec string
			var snd string
			for _, attr := range ev.Attributes {
				switch attr.Key {
				case "amount":
					amt = attr.Value
				case "recipient":
					rec = attr.Value
				case "sender":
					snd = attr.Value
				}
			}
			if rec != "" {
				if parsed, ok := parseAmountForDenom(amt, denom); ok {
					out = append(out, Transfer{Recipient: rec, Amount: parsed, Sender: snd})
				}
			}
		case "coin_received":
			var amt string
			var rec string
			for _, attr := range ev.Attributes {
				switch attr.Key {
				case "amount":
					amt = attr.Value
				case "receiver":
					rec = attr.Value
				}
			}
			if rec != "" {
				if parsed, ok := parseAmountForDenom(amt, denom); ok {
					out = append(out, Transfer{Recipient: rec, Amount: parsed})
				}
			}
		}
	}
	return out
}

func ApplyPayment(ctx context.Context, st *store.Store, order *models.Order, tx chain.Tx, amount string, sender string) (models.OrderStatus, bool, error) {
	paidAt := tx.Timestamp
	if paidAt.IsZero() {
		paidAt = time.Now().UTC()
	}

	cmp := CompareAmount(amount, order.AmountPeaka)
	status := models.OrderPaid
	var creditIssued *int64

	switch {
	case cmp < 0:
		status = models.OrderUnderpaid
	case cmp > 0:
		status = models.OrderOverpaid
	default:
		if paidAt.After(order.ExpiresAt) {
			status = models.OrderLateNoCredit
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
	if err := st.InsertPayment(ctx, payment); err != nil {
		return status, false, err
	}

	updated, err := st.UpdateOrderPayment(ctx, order.OrderID, status, paidAt, tx.Hash, creditIssued)
	if err != nil {
		return status, false, err
	}
	return status, updated > 0, nil
}

func CompareAmount(a, b string) int {
	ai, ok1 := new(big.Int).SetString(a, 10)
	bi, ok2 := new(big.Int).SetString(b, 10)
	if !ok1 || !ok2 {
		return 0
	}
	return ai.Cmp(bi)
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
