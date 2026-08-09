package gsheets

import (
	"context"
	"sort"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
)

// TransactionRepository menulis dan membaca sheet "Transactions".
// Sheet ini bersifat append-only: satu item terjual = satu baris.
type TransactionRepository struct {
	p *Provider
}

// NewTransactionRepository membuat repository transaksi berbasis Sheets.
func NewTransactionRepository(p *Provider) *TransactionRepository {
	return &TransactionRepository{p: p}
}

var _ domain.TransactionRepository = (*TransactionRepository)(nil)

// Append menambahkan baris-baris transaksi baru.
func (r *TransactionRepository) Append(ctx context.Context, tenantID string, lines []domain.TransactionLine) error {
	if len(lines) == 0 {
		return nil
	}
	lock := r.p.Lock(tenantID)
	lock.Lock()
	defer lock.Unlock()

	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return err
	}
	rows := make([][]any, 0, len(lines))
	for _, l := range lines {
		rows = append(rows, transactionLineToRow(l))
	}
	if err := sh.Append(ctx, SheetTransactions, rows); err != nil {
		return apperr.Upstream("gagal menyimpan transaksi").WithCause(err)
	}
	return nil
}

// ListLines membaca baris transaksi pada rentang waktu tertentu, terurut dari
// yang terbaru.
func (r *TransactionRepository) ListLines(ctx context.Context, tenantID string, from, to time.Time) ([]domain.TransactionLine, error) {
	_, sh, _, err := r.p.Clients(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := sh.SheetRows(ctx, SheetTransactions, len(transactionHeader))
	if err != nil {
		return nil, apperr.Upstream("gagal membaca data transaksi").WithCause(err)
	}

	lines := make([]domain.TransactionLine, 0, len(rows))
	for _, row := range rows {
		l := transactionLineFromRow(row)
		if l.TransactionID == "" {
			continue
		}
		if !from.IsZero() && l.Date.Before(from) {
			continue
		}
		if !to.IsZero() && !l.Date.Before(to) {
			continue
		}
		lines = append(lines, l)
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Date.After(lines[j].Date) })
	return lines, nil
}
