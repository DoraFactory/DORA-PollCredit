package main

import (
	"context"
	"log"
	"time"

	"DORAPollCredit/internal/chain"
	"DORAPollCredit/internal/config"
	"DORAPollCredit/internal/db"
	"DORAPollCredit/internal/store"
	"DORAPollCredit/internal/worker"
)

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

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
	rpcEndpoints := cfg.Chain.RPCEndpoints
	if len(rpcEndpoints) == 0 {
		log.Fatalf("rpc_endpoints is empty")
	}

	var rpc chain.Client
	if len(rpcEndpoints) > 1 {
		client, err := chain.NewMultiRPCClient(rpcEndpoints, cfg.Worker.RPCFailoverThreshold)
		if err != nil {
			log.Fatalf("rpc client init failed: %v", err)
		}
		rpc = client
	} else {
		rpc = chain.NewRPCClient(rpcEndpoints[0])
	}
	wsEndpoints := cfg.Chain.WSEndpoints
	if len(wsEndpoints) == 0 {
		for _, rpcEndpoint := range rpcEndpoints {
			if ws := chain.DefaultWSEndpoint(rpcEndpoint); ws != "" {
				wsEndpoints = append(wsEndpoints, ws)
			}
		}
	}
	if len(wsEndpoints) > 0 {
		log.Printf("ws endpoints: %v", wsEndpoints)
	}

	w := &worker.Worker{
		Store:               st,
		Chain:               rpc,
		Denom:               cfg.Chain.Denom,
		Decimals:            cfg.Chain.Decimals,
		ConfirmDepth:        int64(cfg.Chain.ConfirmDepth),
		StartHeight:         cfg.Worker.StartHeight,
		RewindBlocks:        cfg.Worker.RewindBlocks,
		MaxBlocksPerTick:    cfg.Worker.MaxBlocksPerTick,
		PerPage:             cfg.Worker.PerPage,
		Interval:            time.Duration(max64(cfg.Worker.IntervalSeconds, 1)) * time.Second,
		WSEndpoints:         wsEndpoints,
		WSBackfillBlocks:    cfg.Worker.WSBackfillBlocks,
		WSFailoverThreshold: cfg.Worker.WSFailoverThreshold,
	}

	log.Printf("worker started (rpc=%v)", rpcEndpoints)
	w.Run(ctx)
}
