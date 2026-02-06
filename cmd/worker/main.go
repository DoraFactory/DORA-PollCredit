package main

import (
	"context"
	"log"
	"time"

	"DORAPollCredit/internal/chain"
	"DORAPollCredit/internal/config"
	"DORAPollCredit/internal/db"
	"DORAPollCredit/internal/pricing"
	"DORAPollCredit/internal/store"
	"DORAPollCredit/internal/worker"
)

func main() {
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DB.DSN)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	defer pool.Close()

	st := store.New(pool)
	rpc := chain.NewRPCClient(cfg.Chain.RPCEndpoint)
	pricingSvc := pricing.Service{FixedCreditPerDora: cfg.Pricing.FixedCreditPerDora}

	w := &worker.Worker{
		Store:        st,
		Chain:        rpc,
		Pricing:      pricingSvc,
		Denom:        cfg.Chain.Denom,
		Decimals:     cfg.Chain.Decimals,
		ConfirmDepth: int64(cfg.Chain.ConfirmDepth),
		StartHeight:  cfg.Worker.StartHeight,
		Interval:     20 * time.Second,
	}

	log.Printf("worker started (rpc=%s)", cfg.Chain.RPCEndpoint)
	w.Run(ctx)
}
