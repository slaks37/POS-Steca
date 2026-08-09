package service

import (
	"context"
	"strings"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// CartItem adalah satu baris keranjang belanja.
type CartItem struct {
	ProductID string `json:"product_id"`
	Qty       int    `json:"qty"`
	Note      string `json:"note"`
}

// CheckoutInput adalah payload dari layar kasir.
type CheckoutInput struct {
	Items         []CartItem `json:"items"`
	PaymentMethod string     `json:"payment_method"`
	AmountPaid    float64    `json:"amount_paid"`
	CustomerName  string     `json:"customer_name"`
	TableNo       string     `json:"table_no"`
	Note          string     `json:"note"`
}

// OnlineOrderInput adalah payload pemesanan dari kanal online.
type OnlineOrderInput struct {
	Items        []CartItem `json:"items"`
	CustomerName string     `json:"customer_name"`
	TableNo      string     `json:"table_no"`
	Note         string     `json:"note"`
}

// CheckoutResult mengembalikan pesanan sekaligus struk digitalnya.
type CheckoutResult struct {
	Order   domain.Order        `json:"order"`
	Receipt *domain.Transaction `json:"receipt,omitempty"`
}

// OrderService menangani modul kasir dan manajemen pesanan.
type OrderService struct {
	products     domain.ProductRepository
	orders       domain.OrderRepository
	transactions domain.TransactionRepository
}

// NewOrderService membuat service pesanan.
func NewOrderService(
	products domain.ProductRepository,
	orders domain.OrderRepository,
	transactions domain.TransactionRepository,
) *OrderService {
	return &OrderService{products: products, orders: orders, transactions: transactions}
}

// Checkout mencatat penjualan langsung di kasir: stok berkurang, transaksi
// tercatat di sheet "Transactions", dan pesanan masuk antrian dengan status
// "baru" agar bisa dilacak dapur.
func (s *OrderService) Checkout(ctx context.Context, tenantID, cashier string, in CheckoutInput) (*CheckoutResult, error) {
	method := strings.ToLower(strings.TrimSpace(in.PaymentMethod))
	if !domain.ValidPayment(method) {
		return nil, apperr.BadRequest("metode pembayaran harus tunai, qris, atau kartu")
	}
	items, total, err := s.resolveItems(ctx, tenantID, in.Items)
	if err != nil {
		return nil, err
	}
	if method == domain.PaymentTunai && in.AmountPaid > 0 && in.AmountPaid < total {
		return nil, apperr.BadRequest("uang yang dibayarkan kurang dari total belanja")
	}

	now := timex.Now()
	order := &domain.Order{
		ID:            NewOrderID(),
		Code:          NewOrderCode(),
		TransactionID: NewTransactionID(),
		Status:        domain.OrderStatusBaru,
		Source:        domain.OrderSourceKasir,
		CustomerName:  strings.TrimSpace(in.CustomerName),
		TableNo:       strings.TrimSpace(in.TableNo),
		Items:         items,
		Total:         total,
		Note:          strings.TrimSpace(in.Note),
		Cashier:       cashier,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	lines := buildTransactionLines(order, method, cashier, now)
	if err := s.transactions.Append(ctx, tenantID, lines); err != nil {
		return nil, err
	}
	if err := s.orders.Create(ctx, tenantID, order); err != nil {
		return nil, err
	}
	if err := s.products.AdjustStock(ctx, tenantID, stockDeltas(items, -1)); err != nil {
		// Penjualan sudah tercatat; kegagalan sinkronisasi stok tidak boleh
		// membatalkan struk pelanggan.
		return &CheckoutResult{Order: *order, Receipt: buildReceipt(order, method, cashier, in.AmountPaid)}, nil
	}

	return &CheckoutResult{Order: *order, Receipt: buildReceipt(order, method, cashier, in.AmountPaid)}, nil
}

// CreateOnlineOrder mencatat pesanan dari kanal online. Pesanan ini belum
// terbayar: transaksi baru dibuat saat kasir menyelesaikan pembayaran.
func (s *OrderService) CreateOnlineOrder(ctx context.Context, tenantID string, in OnlineOrderInput) (*domain.Order, error) {
	items, total, err := s.resolveItems(ctx, tenantID, in.Items)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.CustomerName) == "" {
		return nil, apperr.BadRequest("nama pemesan wajib diisi")
	}

	now := timex.Now()
	order := &domain.Order{
		ID:           NewOrderID(),
		Code:         NewOrderCode(),
		Status:       domain.OrderStatusBaru,
		Source:       domain.OrderSourceOnline,
		CustomerName: strings.TrimSpace(in.CustomerName),
		TableNo:      strings.TrimSpace(in.TableNo),
		Items:        items,
		Total:        total,
		Note:         strings.TrimSpace(in.Note),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.orders.Create(ctx, tenantID, order); err != nil {
		return nil, err
	}
	if err := s.products.AdjustStock(ctx, tenantID, stockDeltas(items, -1)); err != nil {
		return order, nil
	}
	return order, nil
}

// List mengembalikan pesanan sesuai filter status/sumber.
func (s *OrderService) List(ctx context.Context, tenantID, status, source string, from, to time.Time) ([]domain.Order, error) {
	f := domain.OrderFilter{From: from, To: to}
	if status = strings.ToLower(strings.TrimSpace(status)); status != "" {
		st := domain.OrderStatus(status)
		if !st.Valid() {
			return nil, apperr.BadRequest("status pesanan tidak dikenal")
		}
		f.Status = st
	}
	if source = strings.ToLower(strings.TrimSpace(source)); source != "" {
		src := domain.OrderSource(source)
		if !src.Valid() {
			return nil, apperr.BadRequest("sumber pesanan tidak dikenal")
		}
		f.Source = src
	}
	return s.orders.List(ctx, tenantID, f)
}

// Get mengambil satu pesanan.
func (s *OrderService) Get(ctx context.Context, tenantID, orderID string) (*domain.Order, error) {
	return s.orders.Get(ctx, tenantID, orderID)
}

// UpdateStatus memindahkan pesanan pada alur baru -> diproses -> selesai.
// Pembatalan mengembalikan stok yang sempat dipotong.
func (s *OrderService) UpdateStatus(ctx context.Context, tenantID, orderID, status string) (*domain.Order, error) {
	next := domain.OrderStatus(strings.ToLower(strings.TrimSpace(status)))
	if !next.Valid() {
		return nil, apperr.BadRequest("status pesanan tidak dikenal")
	}
	order, err := s.orders.Get(ctx, tenantID, orderID)
	if err != nil {
		return nil, err
	}
	if order.Status == next {
		return order, nil
	}
	if order.Status == domain.OrderStatusBatal {
		return nil, apperr.Conflict("pesanan yang sudah dibatalkan tidak bisa diubah")
	}
	if next == domain.OrderStatusBatal && order.TransactionID != "" {
		return nil, apperr.Conflict("pesanan yang sudah dibayar tidak bisa dibatalkan")
	}

	previous := order.Status
	order.Status = next
	order.UpdatedAt = timex.Now()
	if err := s.orders.Update(ctx, tenantID, order); err != nil {
		return nil, err
	}
	if next == domain.OrderStatusBatal && previous != domain.OrderStatusBatal {
		_ = s.products.AdjustStock(ctx, tenantID, stockDeltas(order.Items, 1))
	}
	return order, nil
}

// Settle menyelesaikan pembayaran pesanan yang belum terbayar (umumnya
// pesanan online) dan menerbitkan struk.
func (s *OrderService) Settle(ctx context.Context, tenantID, orderID, cashier, paymentMethod string, amountPaid float64) (*CheckoutResult, error) {
	method := strings.ToLower(strings.TrimSpace(paymentMethod))
	if !domain.ValidPayment(method) {
		return nil, apperr.BadRequest("metode pembayaran harus tunai, qris, atau kartu")
	}
	order, err := s.orders.Get(ctx, tenantID, orderID)
	if err != nil {
		return nil, err
	}
	if order.TransactionID != "" {
		return nil, apperr.Conflict("pesanan ini sudah dibayar")
	}
	if order.Status == domain.OrderStatusBatal {
		return nil, apperr.Conflict("pesanan sudah dibatalkan")
	}
	if method == domain.PaymentTunai && amountPaid > 0 && amountPaid < order.Total {
		return nil, apperr.BadRequest("uang yang dibayarkan kurang dari total pesanan")
	}

	now := timex.Now()
	order.TransactionID = NewTransactionID()
	order.Cashier = cashier
	order.UpdatedAt = now

	if err := s.transactions.Append(ctx, tenantID, buildTransactionLines(order, method, cashier, now)); err != nil {
		return nil, err
	}
	if err := s.orders.Update(ctx, tenantID, order); err != nil {
		return nil, err
	}
	return &CheckoutResult{Order: *order, Receipt: buildReceipt(order, method, cashier, amountPaid)}, nil
}

// resolveItems memvalidasi keranjang terhadap katalog dan menghitung total.
func (s *OrderService) resolveItems(ctx context.Context, tenantID string, cart []CartItem) ([]domain.OrderItem, float64, error) {
	if len(cart) == 0 {
		return nil, 0, apperr.BadRequest("keranjang belanja masih kosong")
	}
	catalog, err := s.products.List(ctx, tenantID)
	if err != nil {
		return nil, 0, err
	}
	index := make(map[string]domain.Product, len(catalog))
	for _, p := range catalog {
		index[p.ID] = p
	}

	merged := map[string]*domain.OrderItem{}
	order := []string{}
	for _, c := range cart {
		if c.Qty <= 0 {
			return nil, 0, apperr.BadRequest("jumlah item harus lebih dari nol")
		}
		p, ok := index[c.ProductID]
		if !ok {
			return nil, 0, apperr.BadRequest("produk %s tidak ditemukan di katalog", c.ProductID)
		}
		if existing, ok := merged[c.ProductID]; ok {
			existing.Qty += c.Qty
			if note := strings.TrimSpace(c.Note); note != "" {
				existing.Note = strings.TrimSpace(existing.Note + "; " + note)
			}
			continue
		}
		merged[c.ProductID] = &domain.OrderItem{
			ProductID: p.ID,
			Name:      p.Name,
			Qty:       c.Qty,
			UnitPrice: p.Price,
			Note:      strings.TrimSpace(c.Note),
		}
		order = append(order, c.ProductID)
	}

	items := make([]domain.OrderItem, 0, len(order))
	var total float64
	for _, id := range order {
		item := *merged[id]
		if p := index[id]; p.Stock < item.Qty {
			return nil, 0, apperr.Conflict("stok %s tinggal %d, tidak cukup untuk %d item", p.Name, p.Stock, item.Qty)
		}
		total += item.Subtotal()
		items = append(items, item)
	}
	return items, total, nil
}

func buildTransactionLines(o *domain.Order, method, cashier string, at time.Time) []domain.TransactionLine {
	lines := make([]domain.TransactionLine, 0, len(o.Items))
	for _, item := range o.Items {
		lines = append(lines, domain.TransactionLine{
			Date:          at,
			TransactionID: o.TransactionID,
			Item:          item.Name,
			Qty:           item.Qty,
			UnitPrice:     item.UnitPrice,
			Total:         item.Subtotal(),
			PaymentMethod: method,
			Cashier:       cashier,
		})
	}
	return lines
}

func buildReceipt(o *domain.Order, method, cashier string, amountPaid float64) *domain.Transaction {
	change := 0.0
	if method == domain.PaymentTunai && amountPaid > o.Total {
		change = amountPaid - o.Total
	}
	return &domain.Transaction{
		ID:            o.TransactionID,
		Date:          o.UpdatedAt,
		Items:         o.Items,
		Total:         o.Total,
		PaymentMethod: method,
		Cashier:       cashier,
		AmountPaid:    amountPaid,
		Change:        change,
	}
}

// stockDeltas mengubah daftar item menjadi peta perubahan stok. sign -1 untuk
// mengurangi (penjualan) dan +1 untuk mengembalikan (pembatalan).
func stockDeltas(items []domain.OrderItem, sign int) map[string]int {
	deltas := make(map[string]int, len(items))
	for _, item := range items {
		deltas[item.ProductID] += sign * item.Qty
	}
	return deltas
}
