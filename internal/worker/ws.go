package worker

import (
	"context"
	"errors"
	"log"
	"time"

	"DORAPollCredit/internal/chain"

	"github.com/jackc/pgx/v5"
)

func (w *Worker) RunWS(ctx context.Context) {
	if w.WSEndpoint == "" {
		log.Printf("ws disabled: ws_endpoint is empty")
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		client := chain.NewWSClient(w.WSEndpoint)
		if err := client.Connect(ctx); err != nil {
			log.Printf("ws connect failed: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		log.Printf("ws connected %s", w.WSEndpoint)

		if err := client.Subscribe(ctx, "tm.event='Tx'"); err != nil {
			log.Printf("ws subscribe failed: %v", err)
			client.Close()
			time.Sleep(3 * time.Second)
			continue
		}

		for {
			msg, err := client.Read(ctx)
			if err != nil {
				log.Printf("ws read failed: %v", err)
				client.Close()
				break
			}

			tx, ok, err := chain.ParseWSTx(msg)
			if err != nil {
				log.Printf("ws parse failed: %v", err)
				continue
			}
			if !ok || tx.Code != 0 || tx.Hash == "" {
				continue
			}

			for _, t := range extractTransfers(tx.Events, w.Denom) {
				order, err := w.Store.GetPendingOrderByRecipient(ctx, t.Recipient)
				if err != nil {
					if errors.Is(err, pgx.ErrNoRows) {
						continue
					}
					log.Printf("ws get order failed: %v", err)
					continue
				}
				if err := w.applyPayment(ctx, order, *tx, t.Amount, t.Sender); err != nil {
					log.Printf("ws apply payment failed: %v", err)
				}
			}
		}

		time.Sleep(2 * time.Second)
	}
}
