package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/slaks37/pos-steca/backend/internal/apperr"
	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// Granularity menentukan pengelompokan laporan penjualan.
type Granularity string

const (
	// GranularityHarian mengelompokkan omzet per hari.
	GranularityHarian Granularity = "harian"
	// GranularityMingguan mengelompokkan omzet per minggu (mulai Senin).
	GranularityMingguan Granularity = "mingguan"
	// GranularityBulanan mengelompokkan omzet per bulan.
	GranularityBulanan Granularity = "bulanan"
)

// SeriesPoint adalah satu titik pada grafik omzet.
type SeriesPoint struct {
	Label        string    `json:"label"`
	Start        time.Time `json:"start"`
	Omzet        float64   `json:"omzet"`
	Transactions int       `json:"transactions"`
	Items        int       `json:"items"`
}

// ProductSales merangkum penjualan satu produk.
type ProductSales struct {
	Name  string  `json:"name"`
	Qty   int     `json:"qty"`
	Omzet float64 `json:"omzet"`
}

// PaymentShare merangkum omzet per metode pembayaran.
type PaymentShare struct {
	Method       string  `json:"method"`
	Omzet        float64 `json:"omzet"`
	Transactions int     `json:"transactions"`
}

// CashierSales merangkum omzet per kasir.
type CashierSales struct {
	Cashier      string  `json:"cashier"`
	Omzet        float64 `json:"omzet"`
	Transactions int     `json:"transactions"`
}

// SalesReport adalah laporan penjualan lengkap untuk satu rentang waktu.
type SalesReport struct {
	Granularity       Granularity    `json:"granularity"`
	From              time.Time      `json:"from"`
	To                time.Time      `json:"to"`
	TotalOmzet        float64        `json:"total_omzet"`
	TotalTransactions int            `json:"total_transactions"`
	TotalItems        int            `json:"total_items"`
	AverageBasket     float64        `json:"average_basket"`
	Series            []SeriesPoint  `json:"series"`
	TopProducts       []ProductSales `json:"top_products"`
	Payments          []PaymentShare `json:"payments"`
	Cashiers          []CashierSales `json:"cashiers"`
}

// ReportService membangun laporan dari baris transaksi.
type ReportService struct {
	transactions domain.TransactionRepository
	products     domain.ProductRepository
	orders       domain.OrderRepository
}

// NewReportService membuat service laporan.
func NewReportService(
	transactions domain.TransactionRepository,
	products domain.ProductRepository,
	orders domain.OrderRepository,
) *ReportService {
	return &ReportService{transactions: transactions, products: products, orders: orders}
}

// Sales menyusun laporan penjualan pada rentang [from, to).
func (s *ReportService) Sales(ctx context.Context, tenantID string, g Granularity, from, to time.Time) (*SalesReport, error) {
	switch g {
	case GranularityHarian, GranularityMingguan, GranularityBulanan:
	case "":
		g = GranularityHarian
	default:
		return nil, apperr.BadRequest("granularitas harus harian, mingguan, atau bulanan")
	}
	if from.IsZero() {
		from = timex.StartOfDay(timex.Now()).AddDate(0, 0, -29)
	}
	if to.IsZero() {
		to = timex.StartOfDay(timex.Now()).AddDate(0, 0, 1)
	}
	if !to.After(from) {
		return nil, apperr.BadRequest("tanggal akhir harus setelah tanggal mulai")
	}

	lines, err := s.transactions.ListLines(ctx, tenantID, from, to)
	if err != nil {
		return nil, err
	}
	report := aggregateSales(lines, g)
	report.From = from
	report.To = to
	return report, nil
}

// aggregateSales adalah inti perhitungan laporan; dipisah agar mudah diuji
// tanpa perlu repository.
func aggregateSales(lines []domain.TransactionLine, g Granularity) *SalesReport {
	report := &SalesReport{
		Granularity: g,
		Series:      []SeriesPoint{},
		TopProducts: []ProductSales{},
		Payments:    []PaymentShare{},
		Cashiers:    []CashierSales{},
	}

	type bucket struct {
		start  time.Time
		omzet  float64
		items  int
		trxIDs map[string]bool
	}
	buckets := map[string]*bucket{}
	products := map[string]*ProductSales{}
	payments := map[string]*PaymentShare{}
	paymentTrx := map[string]map[string]bool{}
	cashiers := map[string]*CashierSales{}
	cashierTrx := map[string]map[string]bool{}
	allTrx := map[string]bool{}

	for _, l := range lines {
		if l.Date.IsZero() {
			continue
		}
		start := bucketStart(l.Date, g)
		key := start.Format(time.RFC3339)
		b, ok := buckets[key]
		if !ok {
			b = &bucket{start: start, trxIDs: map[string]bool{}}
			buckets[key] = b
		}
		b.omzet += l.Total
		b.items += l.Qty
		b.trxIDs[l.TransactionID] = true

		p, ok := products[l.Item]
		if !ok {
			p = &ProductSales{Name: l.Item}
			products[l.Item] = p
		}
		p.Qty += l.Qty
		p.Omzet += l.Total

		method := l.PaymentMethod
		if method == "" {
			method = "lainnya"
		}
		pay, ok := payments[method]
		if !ok {
			pay = &PaymentShare{Method: method}
			payments[method] = pay
			paymentTrx[method] = map[string]bool{}
		}
		pay.Omzet += l.Total
		paymentTrx[method][l.TransactionID] = true

		cashier := l.Cashier
		if cashier == "" {
			cashier = "tanpa nama"
		}
		c, ok := cashiers[cashier]
		if !ok {
			c = &CashierSales{Cashier: cashier}
			cashiers[cashier] = c
			cashierTrx[cashier] = map[string]bool{}
		}
		c.Omzet += l.Total
		cashierTrx[cashier][l.TransactionID] = true

		allTrx[l.TransactionID] = true
		report.TotalOmzet += l.Total
		report.TotalItems += l.Qty
	}

	report.TotalTransactions = len(allTrx)
	if report.TotalTransactions > 0 {
		report.AverageBasket = report.TotalOmzet / float64(report.TotalTransactions)
	}

	for _, b := range buckets {
		report.Series = append(report.Series, SeriesPoint{
			Label:        bucketLabel(b.start, g),
			Start:        b.start,
			Omzet:        b.omzet,
			Transactions: len(b.trxIDs),
			Items:        b.items,
		})
	}
	sort.Slice(report.Series, func(i, j int) bool { return report.Series[i].Start.Before(report.Series[j].Start) })

	for _, p := range products {
		report.TopProducts = append(report.TopProducts, *p)
	}
	sort.Slice(report.TopProducts, func(i, j int) bool {
		if report.TopProducts[i].Qty != report.TopProducts[j].Qty {
			return report.TopProducts[i].Qty > report.TopProducts[j].Qty
		}
		return report.TopProducts[i].Omzet > report.TopProducts[j].Omzet
	})

	for method, p := range payments {
		p.Transactions = len(paymentTrx[method])
		report.Payments = append(report.Payments, *p)
	}
	sort.Slice(report.Payments, func(i, j int) bool { return report.Payments[i].Omzet > report.Payments[j].Omzet })

	for name, c := range cashiers {
		c.Transactions = len(cashierTrx[name])
		report.Cashiers = append(report.Cashiers, *c)
	}
	sort.Slice(report.Cashiers, func(i, j int) bool { return report.Cashiers[i].Omzet > report.Cashiers[j].Omzet })

	return report
}

func bucketStart(t time.Time, g Granularity) time.Time {
	switch g {
	case GranularityMingguan:
		return timex.StartOfWeek(t)
	case GranularityBulanan:
		return timex.StartOfMonth(t)
	default:
		return timex.StartOfDay(t)
	}
}

var monthNames = []string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

func bucketLabel(start time.Time, g Granularity) string {
	switch g {
	case GranularityMingguan:
		end := start.AddDate(0, 0, 6)
		return start.Format("2 ") + monthNames[int(start.Month())-1] + " - " + end.Format("2 ") + monthNames[int(end.Month())-1]
	case GranularityBulanan:
		return monthNames[int(start.Month())-1] + start.Format(" 2006")
	default:
		return start.Format("2 ") + monthNames[int(start.Month())-1]
	}
}

// TransactionHistory menggabungkan baris menjadi daftar transaksi utuh.
func (s *ReportService) TransactionHistory(ctx context.Context, tenantID string, from, to time.Time, limit int) ([]domain.Transaction, error) {
	lines, err := s.transactions.ListLines(ctx, tenantID, from, to)
	if err != nil {
		return nil, err
	}
	return groupTransactions(lines, limit), nil
}

func groupTransactions(lines []domain.TransactionLine, limit int) []domain.Transaction {
	index := map[string]*domain.Transaction{}
	order := []string{}
	for _, l := range lines {
		trx, ok := index[l.TransactionID]
		if !ok {
			trx = &domain.Transaction{
				ID:            l.TransactionID,
				Date:          l.Date,
				PaymentMethod: l.PaymentMethod,
				Cashier:       l.Cashier,
			}
			index[l.TransactionID] = trx
			order = append(order, l.TransactionID)
		}
		trx.Items = append(trx.Items, domain.OrderItem{
			Name:      l.Item,
			Qty:       l.Qty,
			UnitPrice: l.UnitPrice,
		})
		trx.Total += l.Total
		if l.Date.After(trx.Date) {
			trx.Date = l.Date
		}
	}

	out := make([]domain.Transaction, 0, len(order))
	for _, id := range order {
		out = append(out, *index[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Date.After(out[j].Date) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// DashboardSummary adalah ringkasan performa toko untuk pemilik.
type DashboardSummary struct {
	BusinessName      string           `json:"business_name"`
	Today             PeriodSummary    `json:"today"`
	Month             PeriodSummary    `json:"month"`
	Trend7Days        []SeriesPoint    `json:"trend_7_days"`
	TopProductsMonth  []ProductSales   `json:"top_products_month"`
	OrdersByStatus    map[string]int   `json:"orders_by_status"`
	ActiveOrders      []domain.Order   `json:"active_orders"`
	LowStockProducts  []domain.Product `json:"low_stock_products"`
	ProductCount      int              `json:"product_count"`
	LowStockThreshold int              `json:"low_stock_threshold"`
}

// PeriodSummary merangkum satu periode waktu.
type PeriodSummary struct {
	Omzet         float64 `json:"omzet"`
	Transactions  int     `json:"transactions"`
	Items         int     `json:"items"`
	AverageBasket float64 `json:"average_basket"`
}

// LowStockThreshold adalah batas stok yang dianggap menipis.
const LowStockThreshold = 5

// Dashboard menyusun ringkasan performa toko.
func (s *ReportService) Dashboard(ctx context.Context, tenantID, businessName string) (*DashboardSummary, error) {
	now := timex.Now()
	monthStart := timex.StartOfMonth(now)
	todayStart := timex.StartOfDay(now)
	tomorrow := todayStart.AddDate(0, 0, 1)
	weekStart := todayStart.AddDate(0, 0, -6)

	// Satu pembacaan rentang bulan berjalan cukup untuk seluruh ringkasan,
	// sehingga dashboard hanya menyentuh database sekali.
	rangeStart := monthStart
	if weekStart.Before(rangeStart) {
		rangeStart = weekStart
	}
	lines, err := s.transactions.ListLines(ctx, tenantID, rangeStart, tomorrow)
	if err != nil {
		return nil, err
	}

	summary := &DashboardSummary{
		BusinessName:      businessName,
		OrdersByStatus:    map[string]int{},
		LowStockThreshold: LowStockThreshold,
	}

	todayLines := filterLines(lines, todayStart, tomorrow)
	monthLines := filterLines(lines, monthStart, tomorrow)
	summary.Today = summarize(todayLines)
	summary.Month = summarize(monthLines)

	weekReport := aggregateSales(filterLines(lines, weekStart, tomorrow), GranularityHarian)
	summary.Trend7Days = fillDailySeries(weekReport.Series, weekStart, 7)

	monthReport := aggregateSales(monthLines, GranularityHarian)
	summary.TopProductsMonth = topN(monthReport.TopProducts, 5)

	orders, err := s.orders.List(ctx, tenantID, domain.OrderFilter{From: todayStart.AddDate(0, 0, -1)})
	if err != nil {
		return nil, err
	}
	active := []domain.Order{}
	for _, o := range orders {
		summary.OrdersByStatus[string(o.Status)]++
		if o.Status == domain.OrderStatusBaru || o.Status == domain.OrderStatusDiproses {
			active = append(active, o)
		}
	}
	if len(active) > 10 {
		active = active[:10]
	}
	summary.ActiveOrders = active

	products, err := s.products.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	summary.ProductCount = len(products)
	low := []domain.Product{}
	for _, p := range products {
		if p.Stock <= LowStockThreshold {
			low = append(low, p)
		}
	}
	sort.Slice(low, func(i, j int) bool { return low[i].Stock < low[j].Stock })
	if len(low) > 10 {
		low = low[:10]
	}
	summary.LowStockProducts = low

	return summary, nil
}

func filterLines(lines []domain.TransactionLine, from, to time.Time) []domain.TransactionLine {
	out := make([]domain.TransactionLine, 0, len(lines))
	for _, l := range lines {
		if l.Date.IsZero() || l.Date.Before(from) || !l.Date.Before(to) {
			continue
		}
		out = append(out, l)
	}
	return out
}

func summarize(lines []domain.TransactionLine) PeriodSummary {
	var s PeriodSummary
	trx := map[string]bool{}
	for _, l := range lines {
		s.Omzet += l.Total
		s.Items += l.Qty
		trx[l.TransactionID] = true
	}
	s.Transactions = len(trx)
	if s.Transactions > 0 {
		s.AverageBasket = s.Omzet / float64(s.Transactions)
	}
	return s
}

// fillDailySeries memastikan grafik tetap punya titik untuk hari tanpa
// penjualan, agar tren terbaca utuh di frontend.
func fillDailySeries(series []SeriesPoint, start time.Time, days int) []SeriesPoint {
	index := map[string]SeriesPoint{}
	for _, p := range series {
		index[timex.StartOfDay(p.Start).Format("2006-01-02")] = p
	}
	out := make([]SeriesPoint, 0, days)
	for i := 0; i < days; i++ {
		day := timex.StartOfDay(start).AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		if p, ok := index[key]; ok {
			out = append(out, p)
			continue
		}
		out = append(out, SeriesPoint{Label: bucketLabel(day, GranularityHarian), Start: day})
	}
	return out
}

func topN(items []ProductSales, n int) []ProductSales {
	if len(items) > n {
		return items[:n]
	}
	return items
}

// ParseGranularity membaca parameter granularitas dari query string.
func ParseGranularity(raw string) Granularity {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "mingguan", "weekly", "week":
		return GranularityMingguan
	case "bulanan", "monthly", "month":
		return GranularityBulanan
	default:
		return GranularityHarian
	}
}
