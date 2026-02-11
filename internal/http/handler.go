package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"DORAPollCredit/internal/chain"
	"DORAPollCredit/internal/models"
	"DORAPollCredit/internal/payments"
	"DORAPollCredit/internal/services"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type Handler struct {
	Orders *services.OrderService
	Chain  chain.Client
}

type createOrderRequest struct {
	Credit int64 `json:"credit"`
}

type createOrderResponse struct {
	OrderID          string          `json:"orderId"`
	AmountPeaka      string          `json:"amountPeaka"`
	Denom            string          `json:"denom"`
	RecipientAddress string          `json:"recipientAddress"`
	ExpiresAt        string          `json:"expiresAt"`
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

type adminOrderResponse struct {
	OrderID          string `json:"orderId"`
	UserID           string `json:"userId"`
	Status           string `json:"status"`
	AmountPeaka      string `json:"amountPeaka"`
	Denom            string `json:"denom"`
	RecipientAddress string `json:"recipientAddress"`
	ExpiresAt        string `json:"expiresAt"`
	PaidAt           string `json:"paidAt,omitempty"`
	TxHash           string `json:"txHash,omitempty"`
	CreditIssued     *int64 `json:"creditIssued,omitempty"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

func NewHandler(orders *services.OrderService, chainClient chain.Client) *Handler {
	return &Handler{Orders: orders, Chain: chainClient}
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

type adminVerifyTxRequest struct {
	TxHash string `json:"txHash"`
}

func (h *Handler) AdminVerifyTx(w http.ResponseWriter, r *http.Request) {
	if h.Chain == nil {
		writeError(w, http.StatusPreconditionFailed, "rpc client not configured")
		return
	}

	var req adminVerifyTxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.TxHash == "" {
		writeError(w, http.StatusBadRequest, "missing txHash")
		return
	}

	tx, err := h.Chain.TxByHash(r.Context(), req.TxHash)
	if err != nil {
		writeError(w, http.StatusBadRequest, "tx query failed")
		return
	}
	if tx.Code != 0 {
		writeError(w, http.StatusBadRequest, "tx failed")
		return
	}
	if tx.Timestamp.IsZero() {
		if t, err := h.Chain.BlockTime(r.Context(), tx.Height); err == nil {
			tx.Timestamp = t
		} else {
			tx.Timestamp = time.Now().UTC()
		}
	}

	transfers := payments.ExtractTransfers(tx.Events, h.Orders.Denom)
	if len(transfers) == 0 {
		writeError(w, http.StatusBadRequest, "no transfer for denom")
		return
	}

	updatedItems := make([]adminOrderResponse, 0)
	for _, t := range transfers {
		order, err := h.Orders.GetOrderByRecipient(r.Context(), t.Recipient)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			writeError(w, http.StatusInternalServerError, "get order failed")
			return
		}
		if order.Status != models.OrderCreated && order.Status != models.OrderExpired {
			continue
		}

		status, updated, err := h.Orders.ApplyPayment(r.Context(), order, *tx, t.Amount, t.Sender)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "apply payment failed")
			return
		}
		if !updated {
			continue
		}
		var creditIssued *int64
		if status == models.OrderPaid {
			creditIssued = &order.CreditRequested
		}
		resp := adminOrderResponse{
			OrderID:          order.OrderID,
			UserID:           order.UserID,
			Status:           string(status),
			AmountPeaka:      order.AmountPeaka,
			Denom:            order.Denom,
			RecipientAddress: order.RecipientAddress,
			ExpiresAt:        order.ExpiresAt.Format(time.RFC3339),
			CreditIssued:     creditIssued,
			CreatedAt:        order.CreatedAt.Format(time.RFC3339),
			UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
		}
		resp.PaidAt = tx.Timestamp.Format(time.RFC3339)
		resp.TxHash = tx.Hash
		updatedItems = append(updatedItems, resp)
	}

	if len(updatedItems) == 0 {
		writeError(w, http.StatusNotFound, "no matching order")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items": updatedItems,
	})
}

func parseQueryInt(r *http.Request, key string, fallback int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func (h *Handler) AdminListOrders(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	limit := parseQueryInt(r, "limit", 50)
	offset := parseQueryInt(r, "offset", 0)

	orders, err := h.Orders.ListOrdersByStatus(r.Context(), status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list orders failed")
		return
	}

	items := make([]adminOrderResponse, 0, len(orders))
	for _, order := range orders {
		item := adminOrderResponse{
			OrderID:          order.OrderID,
			UserID:           order.UserID,
			Status:           string(order.Status),
			AmountPeaka:      order.AmountPeaka,
			Denom:            order.Denom,
			RecipientAddress: order.RecipientAddress,
			ExpiresAt:        order.ExpiresAt.Format(time.RFC3339),
			CreditIssued:     order.CreditIssued,
			CreatedAt:        order.CreatedAt.Format(time.RFC3339),
			UpdatedAt:        order.UpdatedAt.Format(time.RFC3339),
		}
		if order.PaidAt != nil {
			item.PaidAt = order.PaidAt.Format(time.RFC3339)
		}
		if order.TxHash != nil {
			item.TxHash = *order.TxHash
		}
		items = append(items, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  items,
		"limit":  limit,
		"offset": offset,
	})
}

func (h *Handler) AdminGetOrder(w http.ResponseWriter, r *http.Request) {
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

	resp := adminOrderResponse{
		OrderID:          order.OrderID,
		UserID:           order.UserID,
		Status:           string(order.Status),
		AmountPeaka:      order.AmountPeaka,
		Denom:            order.Denom,
		RecipientAddress: order.RecipientAddress,
		ExpiresAt:        order.ExpiresAt.Format(time.RFC3339),
		CreditIssued:     order.CreditIssued,
		CreatedAt:        order.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        order.UpdatedAt.Format(time.RFC3339),
	}
	if order.PaidAt != nil {
		resp.PaidAt = order.PaidAt.Format(time.RFC3339)
	}
	if order.TxHash != nil {
		resp.TxHash = *order.TxHash
	}

	writeJSON(w, http.StatusOK, resp)
}
