package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/slaks37/pos-steca/backend/internal/domain"
)

const transactionColumns = `occurred_at, transaction_id, item, qty,
	unit_price::float8, total::float8, payment_method, cashier`

// TransactionRepository memetakan tabel transaction_lines yang bersifat
// append-only: satu baris untuk setiap item terjual.
type TransactionRepository struct {
	pool *pgxpool.Pool
}

// NewTransactionRepository membuat repository transaksi berbasis PostgreSQL.
func NewTransactionRepository(pool *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{pool: pool}
}

var _ domain.TransactionRepository = (*TransactionRepository)(nil)

// Append menambahkan baris-baris transaksi baru dalam satu operasi.
func (r *TransactionRepository) Append(ctx context.Context, tenantID string, lines []domain.TransactionLine) error {
	if len(lines) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(lines))
	for _, l := range lines {
		occurred := l.Date
		if occurred.IsZero() {
			occurred = time.Now()
		}
		rows = append(rows, []any{
			tenantID, occurred.UTC(), l.TransactionID, l.Item,
			l.Qty, l.UnitPrice, l.Total, l.PaymentMethod, l.Cashier,
		})
	}

	_, err := r.pool.CopyFrom(ctx,
		pgx.Identifier{"transaction_lines"},
		[]string{"tenant_id", "occurred_at", "transaction_id", "item",
			"qty", "unit_price", "total", "payment_method", "cashier"},
		pgx.CopyFromRows(rows))
	if err != nil {
		return wrapDB(err, "transaksi")
	}
	return nil
}

// ListLines membaca baris transaksi pada rentang [from, to), terbaru dulu.
// Nilai nol pada from/to berarti tanpa batas.
func (r *TransactionRepository) ListLines(
	ctx context.Context, tenantID string, from, to time.Time,
) ([]domain.TransactionLine, error) {
	// Penyaringan rentang dilakukan di database agar laporan bulanan tidak
	// perlu menarik seluruh riwayat ke memori aplikasi.
	rows, err := r.pool.Query(ctx, `
		SELECT `+transactionColumns+`
		FROM transaction_lines
		WHERE tenant_id = $1
		  AND ($2::timestamptz IS NULL OR occurred_at >= $2)
		  AND ($3::timestamptz IS NULL OR occurred_at < $3)
		ORDER BY occurred_at DESC, seq DESC`,
		tenantID, timePtr(from), timePtr(to))
	if err != nil {
		return nil, wrapDB(err, "transaksi")
	}
	defer rows.Close()

	out := []domain.TransactionLine{}
	for rows.Next() {
		l, err := scanTransactionLine(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapDB(err, "transaksi")
	}
	return out, nil
}

func scanTransactionLine(row rowScanner) (domain.TransactionLine, error) {
	var l domain.TransactionLine
	err := row.Scan(&l.Date, &l.TransactionID, &l.Item, &l.Qty,
		&l.UnitPrice, &l.Total, &l.PaymentMethod, &l.Cashier)
	if err != nil {
		return domain.TransactionLine{}, wrapDB(err, "transaksi")
	}
	l.Date = localTime(l.Date)
	return l, nil
}
