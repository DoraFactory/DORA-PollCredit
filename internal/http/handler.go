package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"DORAPollCredit/internal/services"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type Handler struct {
	Orders *services.OrderService
}

type createOrderRequest struct {
	Credit int64 `json:"credit"`
}

type createOrderResponse struct {
	OrderID          string `json:"orderId"`
	AmountPeaka      string `json:"amountPeaka"`
	Denom            string `json:"denom"`
	RecipientAddress string `json:"recipientAddress"`
	ExpiresAt        string `json:"expiresAt"`
	PriceSnapshot    json.RawMessage `json:"priceSnapshot"`
}

type orderResponse struct {
	Status           string `json:"status"`
	AmountPeaka      string `json:"amountPeaka"`
	Denom            string `json:"denom"`
	RecipientAddress string `json:"recipientAddress"`
	ExpiresAt        string `json:"expiresAt,omitempty"`
	PaidAt           string `json:"paidAt,omitempty"`
	TxHash           string `json:"txHash,omitempty"`
	CreditIssued     *int64 `json:"creditIssued,omitempty"`
}

func NewHandler(orders *services.OrderService) *Handler {
	return &Handler{Orders: orders}
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	userID := r.Header.Get("X-User-Id")
	order, err := h.Orders.CreateOrder(r.Context(), userID, req.Credit)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrMissingUserID):
			writeError(w, http.StatusUnauthorized, "missing user id")
		case errors.Is(err, services.ErrInvalidCredit):
			writeError(w, http.StatusBadRequest, "credit below minimum")
		case errors.Is(err, services.ErrXpubNotConfigured):
			writeError(w, http.StatusPreconditionFailed, "wallet xpub not configured")
		default:
			writeError(w, http.StatusInternalServerError, "create order failed")
		}
		return
	}

	resp := createOrderResponse{
		OrderID:          order.OrderID,
		AmountPeaka:      order.AmountPeaka,
		Denom:            order.Denom,
		RecipientAddress: order.RecipientAddress,
		ExpiresAt:        order.ExpiresAt.Format(time.RFC3339),
		PriceSnapshot:    json.RawMessage(order.PriceSnapshot),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	orderID := chi.URLParam(r, "orderId")
	if orderID == "" {
		writeError(w, http.StatusBadRequest, "missing order id")
		return
	}

	order, err := h.Orders.GetOrder(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "order not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get order failed")
		return
	}

	resp := orderResponse{
		Status:           string(order.Status),
		AmountPeaka:      order.AmountPeaka,
		Denom:            order.Denom,
		RecipientAddress: order.RecipientAddress,
		CreditIssued:     order.CreditIssued,
	}
	resp.ExpiresAt = order.ExpiresAt.Format(time.RFC3339)
	if order.PaidAt != nil {
		resp.PaidAt = order.PaidAt.Format(time.RFC3339)
	}
	if order.TxHash != nil {
		resp.TxHash = *order.TxHash
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) ConfirmPayment(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "not implemented")
}
