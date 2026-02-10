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
	if len(w.WSEndpoints) == 0 {
		log.Printf("ws disabled: ws_endpoints is empty")
		return
	}

	backfillBlocks := w.WSBackfillBlocks
	if backfillBlocks <= 0 {
		backfillBlocks = w.RewindBlocks
	}
	if backfillBlocks < 0 {
		backfillBlocks = 0
	}

	failoverThreshold := w.WSFailoverThreshold
	if failoverThreshold <= 0 {
		failoverThreshold = 3
	}

	index := 0
	failCount := 0
	backoff := 2 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		endpoint := w.WSEndpoints[index]
		client := chain.NewWSClient(endpoint)
		if err := client.Connect(ctx); err != nil {
			log.Printf("ws connect failed (%s): %v", endpoint, err)
			failCount++
			if failCount >= failoverThreshold && len(w.WSEndpoints) > 1 {
				index = (index + 1) % len(w.WSEndpoints)
				failCount = 0
				log.Printf("ws failover -> %s", w.WSEndpoints[index])
			}
			time.Sleep(backoff)
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			continue
		}
		log.Printf("ws connected %s", endpoint)
		backoff = 2 * time.Second
		failCount = 0

		if err := client.Subscribe(ctx, "tm.event='Tx'"); err != nil {
			log.Printf("ws subscribe failed (%s): %v", endpoint, err)
			client.Close()
			failCount++
			if failCount >= failoverThreshold && len(w.WSEndpoints) > 1 {
				index = (index + 1) % len(w.WSEndpoints)
				failCount = 0
				log.Printf("ws failover -> %s", w.WSEndpoints[index])
			}
			time.Sleep(backoff)
			continue
		}

		if backfillBlocks > 0 {
			w.BackfillRecent(ctx, backfillBlocks)
		}

		for {
			msg, err := client.Read(ctx)
			if err != nil {
				log.Printf("ws read failed (%s): %v", endpoint, err)
				client.Close()
				failCount++
				if failCount >= failoverThreshold && len(w.WSEndpoints) > 1 {
					index = (index + 1) % len(w.WSEndpoints)
					failCount = 0
					log.Printf("ws failover -> %s", w.WSEndpoints[index])
				}
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

		time.Sleep(backoff)
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}
