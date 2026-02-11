package models

import "time"

type OrderStatus string

const (
	OrderCreated         OrderStatus = "created"
	OrderPaid            OrderStatus = "paid"
	OrderExpired         OrderStatus = "expired"
	OrderPaidLateReprice OrderStatus = "paid_late_repriced"
	OrderLateNoCredit    OrderStatus = "late_no_credit"
	OrderUnderpaid       OrderStatus = "underpaid"
	OrderOverpaid        OrderStatus = "overpaid"
)

type Order struct {
	OrderID          string
	UserID           string
	RecipientAddress string
	DerivationIndex  int64
	CreditRequested  int64
	AmountPeaka      string
	Denom            string
	PriceSnapshot    string
	ExpiresAt        time.Time
	Status           OrderStatus
	PaidAt           *time.Time
	TxHash           *string
	CreditIssued     *int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type Payment struct {
	TxHash      string
	OrderID     string
	FromAddress string
	ToAddress   string
	AmountPeaka string
	Denom       string
	Height      int64
	BlockTime   time.Time
	CreatedAt   time.Time
}
