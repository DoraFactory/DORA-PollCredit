package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"DORAPollCredit/internal/chain"
	"DORAPollCredit/internal/config"
	"DORAPollCredit/internal/db"
	internalhttp "DORAPollCredit/internal/http"
	"DORAPollCredit/internal/pricing"
	"DORAPollCredit/internal/services"
	"DORAPollCredit/internal/store"
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
	pricingSvc := pricing.Service{FixedCreditPerDora: cfg.Pricing.FixedCreditPerDora}
	deriver := chain.AddressDeriver{XPub: cfg.Wallet.XPub, Prefix: cfg.Chain.Bech32Prefix}
	orderSvc := &services.OrderService{
		Store:     st,
		Deriver:   deriver,
		Pricing:   pricingSvc,
		MinCredit: cfg.Orders.MinCredit,
		TTL:       time.Duration(cfg.Orders.TTLMinutes) * time.Minute,
		Denom:     cfg.Chain.Denom,
		Decimals:  cfg.Chain.Decimals,
	}

	h := internalhttp.NewHandler(orderSvc)
	srv := internalhttp.NewServer(h)

	httpServer := &http.Server{
		Addr:    cfg.Server.Addr,
		Handler: srv.Router,
	}

	go func() {
		log.Printf("api listening on %s", cfg.Server.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctxShutdown)
}
