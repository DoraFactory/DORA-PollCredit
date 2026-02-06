package services

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"DORAPollCredit/internal/chain"
	"DORAPollCredit/internal/models"
	"DORAPollCredit/internal/pricing"
	"DORAPollCredit/internal/store"

	"github.com/google/uuid"
)

var (
	ErrMissingUserID   = errors.New("missing user id")
	ErrInvalidCredit   = errors.New("credit below minimum")
	ErrXpubNotConfigured = errors.New("wallet xpub not configured")
)

type OrderService struct {
	Store       *store.Store
	Deriver     chain.AddressDeriver
	Pricing     pricing.Service
	MinCredit   int64
	TTL         time.Duration
	Denom       string
	Decimals    int
}

func (s OrderService) CreateOrder(ctx context.Context, userID string, credit int64) (*models.Order, error) {
	if userID == "" {
		return nil, ErrMissingUserID
	}
	if credit < s.MinCredit {
		return nil, ErrInvalidCredit
	}
	if s.Deriver.XPub == "" {
		return nil, ErrXpubNotConfigured
	}

	snap, err := s.Pricing.CurrentSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	amountPeaka, err := calcAmountPeaka(credit, snap.CreditPerDora, s.Decimals)
	if err != nil {
		return nil, err
	}

	idx, err := s.Store.NextDerivationIndex(ctx)
	if err != nil {
		return nil, err
	}

	addr, err := s.Deriver.Derive(uint32(idx))
	if err != nil {
		return nil, err
	}

	snapJSON, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	order := &models.Order{
		OrderID:          uuid.NewString(),
		UserID:           userID,
		RecipientAddress: addr,
		DerivationIndex:  idx,
		CreditRequested:  credit,
		AmountPeaka:      amountPeaka,
		Denom:            s.Denom,
		PriceSnapshot:    string(snapJSON),
		ExpiresAt:        now.Add(s.TTL),
		Status:           models.OrderCreated,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.Store.CreateOrder(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s OrderService) GetOrder(ctx context.Context, orderID string) (*models.Order, error) {
	return s.Store.GetOrder(ctx, orderID)
}

func calcAmountPeaka(creditRequested int64, creditPerDora int64, decimals int) (string, error) {
	if creditPerDora <= 0 {
		return "", errors.New("credit per dora must be positive")
	}
	if decimals < 0 || decimals > 30 {
		return "", errors.New("invalid decimals")
	}

	pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	num := new(big.Int).Mul(big.NewInt(creditRequested), pow)
	den := big.NewInt(creditPerDora)
	quot, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	if rem.Sign() > 0 {
		quot.Add(quot, big.NewInt(1))
	}
	return quot.String(), nil
}
