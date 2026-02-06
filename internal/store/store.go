package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"DORAPollCredit/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{Pool: pool}
}

func (s *Store) NextDerivationIndex(ctx context.Context) (int64, error) {
	var idx int64
	err := s.Pool.QueryRow(ctx, "SELECT nextval('order_derivation_index_seq')").Scan(&idx)
	return idx, err
}

func (s *Store) CreateOrder(ctx context.Context, order *models.Order) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO orders (
			order_id, user_id, recipient_address, derivation_index,
			credit_requested, amount_peaka, denom, price_snapshot,
			expires_at, status, paid_at, tx_hash, credit_issued
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`,
		order.OrderID,
		order.UserID,
		order.RecipientAddress,
		order.DerivationIndex,
		order.CreditRequested,
		order.AmountPeaka,
		order.Denom,
		order.PriceSnapshot,
		order.ExpiresAt,
		order.Status,
		order.PaidAt,
		order.TxHash,
		order.CreditIssued,
	)
	return err
}

func (s *Store) GetOrder(ctx context.Context, orderID string) (*models.Order, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT order_id, user_id, recipient_address, derivation_index,
			credit_requested, amount_peaka, denom, price_snapshot,
			expires_at, status, paid_at, tx_hash, credit_issued,
			created_at, updated_at
		FROM orders WHERE order_id=$1
	`, orderID)

	var order models.Order
	var paidAt sql.NullTime
	var txHash sql.NullString
	var creditIssued sql.NullInt64

	err := row.Scan(
		&order.OrderID,
		&order.UserID,
		&order.RecipientAddress,
		&order.DerivationIndex,
		&order.CreditRequested,
		&order.AmountPeaka,
		&order.Denom,
		&order.PriceSnapshot,
		&order.ExpiresAt,
		&order.Status,
		&paidAt,
		&txHash,
		&creditIssued,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if paidAt.Valid {
		order.PaidAt = &paidAt.Time
	}
	if txHash.Valid {
		order.TxHash = &txHash.String
	}
	if creditIssued.Valid {
		order.CreditIssued = &creditIssued.Int64
	}
	return &order, nil
}

func (s *Store) GetSyncHeight(ctx context.Context) (int64, error) {
	row := s.Pool.QueryRow(ctx, "SELECT value FROM sync_state WHERE key='last_processed_height'")
	var v string
	if err := row.Scan(&v); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}

func (s *Store) SetSyncHeight(ctx context.Context, height int64) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO sync_state (key, value)
		VALUES ('last_processed_height', $1)
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value
	`, strconv.FormatInt(height, 10))
	return err
}

func (s *Store) ListPendingOrders(ctx context.Context) ([]*models.Order, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT order_id, user_id, recipient_address, derivation_index,
			credit_requested, amount_peaka, denom, price_snapshot,
			expires_at, status, paid_at, tx_hash, credit_issued,
			created_at, updated_at
		FROM orders
		WHERE status IN ('created','expired')
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*models.Order
	for rows.Next() {
		var order models.Order
		var paidAt sql.NullTime
		var txHash sql.NullString
		var creditIssued sql.NullInt64
		if err := rows.Scan(
			&order.OrderID,
			&order.UserID,
			&order.RecipientAddress,
			&order.DerivationIndex,
			&order.CreditRequested,
			&order.AmountPeaka,
			&order.Denom,
			&order.PriceSnapshot,
			&order.ExpiresAt,
			&order.Status,
			&paidAt,
			&txHash,
			&creditIssued,
			&order.CreatedAt,
			&order.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if paidAt.Valid {
			order.PaidAt = &paidAt.Time
		}
		if txHash.Valid {
			order.TxHash = &txHash.String
		}
		if creditIssued.Valid {
			order.CreditIssued = &creditIssued.Int64
		}
		orders = append(orders, &order)
	}
	return orders, rows.Err()
}

func (s *Store) MarkExpired(ctx context.Context, now time.Time) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE orders
		SET status='expired', updated_at=now()
		WHERE status='created' AND expires_at < $1
	`, now)
	return err
}

func (s *Store) InsertPayment(ctx context.Context, payment *models.Payment) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO payments (
			tx_hash, order_id, from_address, to_address,
			amount_peaka, denom, height, block_time
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (tx_hash) DO NOTHING
	`,
		payment.TxHash,
		payment.OrderID,
		payment.FromAddress,
		payment.ToAddress,
		payment.AmountPeaka,
		payment.Denom,
		payment.Height,
		payment.BlockTime,
	)
	return err
}

func (s *Store) UpdateOrderPayment(ctx context.Context, orderID string, status models.OrderStatus, paidAt time.Time, txHash string, creditIssued *int64) (int64, error) {
	res, err := s.Pool.Exec(ctx, `
		UPDATE orders
		SET status=$2, paid_at=$3, tx_hash=$4, credit_issued=$5, updated_at=now()
		WHERE order_id=$1 AND status IN ('created','expired')
	`, orderID, status, paidAt, txHash, creditIssued)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}
