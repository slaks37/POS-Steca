package service

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// TableInput adalah payload pembuatan/perubahan meja.
type TableInput struct {
	Name     string             `json:"name"`
	Capacity int                `json:"capacity"`
	Area     string             `json:"area"`
	Status   domain.TableStatus `json:"status"`
}

// TableService mengelola denah meja bisnis F&B.
type TableService struct {
	tables domain.TableRepository
	orders domain.OrderRepository
}

// NewTableService membuat service meja.
func NewTableService(tables domain.TableRepository, orders domain.OrderRepository) *TableService {
	return &TableService{tables: tables, orders: orders}
}

// List mengembalikan seluruh meja, diurutkan per area lalu nomor meja secara
// natural ("Meja 2" sebelum "Meja 10").
func (s *TableService) List(ctx context.Context, tenantID string) ([]domain.Table, error) {
	items, err := s.tables.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !strings.EqualFold(items[i].Area, items[j].Area) {
			return items[i].Area < items[j].Area
		}
		ni, oki := trailingNumber(items[i].Name)
		nj, okj := trailingNumber(items[j].Name)
		if oki && okj && ni != nj {
			return ni < nj
		}
		return items[i].Name < items[j].Name
	})
	return items, nil
}

// Get mengambil satu meja.
func (s *TableService) Get(ctx context.Context, tenantID, tableID string) (*domain.Table, error) {
	return s.tables.Get(ctx, tenantID, tableID)
}

// Create menambahkan meja baru.
func (s *TableService) Create(ctx context.Context, tenantID string, in TableInput) (*domain.Table, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, apperr.BadRequest("nama atau nomor meja wajib diisi")
	}
	if in.Capacity < 0 {
		return nil, apperr.BadRequest("kapasitas meja tidak boleh negatif")
	}

	existing, err := s.tables.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, tb := range existing {
		if strings.EqualFold(strings.TrimSpace(tb.Name), name) {
			return nil, apperr.Conflict("meja %q sudah ada", name)
		}
	}

	tb := &domain.Table{
		ID:        NewTableID(),
		Name:      name,
		Capacity:  in.Capacity,
		Status:    domain.TableStatusKosong,
		Area:      strings.TrimSpace(in.Area),
		UpdatedAt: timex.Now(),
	}
	if err := s.tables.Create(ctx, tenantID, tb); err != nil {
		return nil, err
	}
	return tb, nil
}

// Update mengubah data meja (nama, kapasitas, area, atau statusnya).
func (s *TableService) Update(ctx context.Context, tenantID, tableID string, in TableInput) (*domain.Table, error) {
	existing, err := s.tables.Get(ctx, tenantID, tableID)
	if err != nil {
		return nil, err
	}
	if name := strings.TrimSpace(in.Name); name != "" {
		existing.Name = name
	}
	if in.Capacity > 0 {
		existing.Capacity = in.Capacity
	}
	existing.Area = strings.TrimSpace(in.Area)
	if in.Status != "" {
		if !in.Status.Valid() {
			return nil, apperr.BadRequest("status meja harus kosong, terisi, atau dibersihkan")
		}
		existing.Status = in.Status
		if in.Status == domain.TableStatusKosong {
			existing.ActiveOrderID = ""
		}
	}
	existing.UpdatedAt = timex.Now()

	if err := s.tables.Update(ctx, tenantID, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

// UpdateStatus mengubah status meja saja, dipakai kasir dari layar transaksi.
func (s *TableService) UpdateStatus(ctx context.Context, tenantID, tableID, status string) (*domain.Table, error) {
	next := domain.TableStatus(strings.ToLower(strings.TrimSpace(status)))
	if !next.Valid() {
		return nil, apperr.BadRequest("status meja harus kosong, terisi, atau dibersihkan")
	}
	existing, err := s.tables.Get(ctx, tenantID, tableID)
	if err != nil {
		return nil, err
	}
	existing.Status = next
	if next == domain.TableStatusKosong {
		existing.ActiveOrderID = ""
	}
	existing.UpdatedAt = timex.Now()

	if err := s.tables.Update(ctx, tenantID, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

// Delete menghapus meja yang tidak sedang dipakai.
func (s *TableService) Delete(ctx context.Context, tenantID, tableID string) error {
	existing, err := s.tables.Get(ctx, tenantID, tableID)
	if err != nil {
		return err
	}
	if existing.Status == domain.TableStatusTerisi {
		return apperr.Conflict("meja %s sedang terisi, selesaikan pesanannya dulu", existing.Name)
	}
	return s.tables.Delete(ctx, tenantID, tableID)
}

// occupy menandai meja terisi oleh sebuah pesanan. Kegagalan di sini tidak
// boleh membatalkan transaksi yang sudah tercatat, jadi errornya dilaporkan
// ke pemanggil untuk sekadar dicatat.
func (s *TableService) occupy(ctx context.Context, tenantID, tableID, orderID string) error {
	if tableID == "" {
		return nil
	}
	tb, err := s.tables.Get(ctx, tenantID, tableID)
	if err != nil {
		return err
	}
	tb.Status = domain.TableStatusTerisi
	tb.ActiveOrderID = orderID
	tb.UpdatedAt = timex.Now()
	return s.tables.Update(ctx, tenantID, tb)
}

// release menandai meja perlu dibersihkan setelah pesanannya selesai atau
// dibatalkan. Meja yang sudah dipakai pesanan lain dibiarkan apa adanya.
func (s *TableService) release(ctx context.Context, tenantID, tableID, orderID string) error {
	if tableID == "" {
		return nil
	}
	tb, err := s.tables.Get(ctx, tenantID, tableID)
	if err != nil {
		return err
	}
	if tb.ActiveOrderID != "" && tb.ActiveOrderID != orderID {
		return nil
	}
	tb.Status = domain.TableStatusDibersihkan
	tb.ActiveOrderID = ""
	tb.UpdatedAt = timex.Now()
	return s.tables.Update(ctx, tenantID, tb)
}

// trailingNumber mengambil angka di akhir nama meja untuk pengurutan natural.
func trailingNumber(name string) (int, bool) {
	end := len(name)
	for end > 0 && name[end-1] >= '0' && name[end-1] <= '9' {
		end--
	}
	if end == len(name) {
		return 0, false
	}
	n, err := strconv.Atoi(name[end:])
	if err != nil {
		return 0, false
	}
	return n, true
}
