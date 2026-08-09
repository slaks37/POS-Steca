package gsheets

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/slaks37/pos-steca/backend/internal/domain"
	"github.com/slaks37/pos-steca/backend/internal/timex"
)

// cell mengambil kolom ke-i dengan aman meski baris lebih pendek dari header
// (Google Sheets memangkas sel kosong di ujung kanan).
func cell(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// parseFloat membaca sel angka. Sel yang ditulis aplikasi selalu berupa angka
// murni, tetapi pemilik usaha bisa saja mengetik sendiri di Google Sheets,
// jadi format rupiah seperti "Rp 15.000" dan "15.000,50" ikut dikenali.
func parseFloat(s string) float64 {
	s = strings.Map(func(r rune) rune {
		// Buang spasi apa pun, termasuk non-breaking space hasil salin-tempel.
		if r == ' ' || r == '\t' || r == '\u00a0' {
			return -1
		}
		return r
	}, s)
	s = strings.NewReplacer("Rp", "", "rp", "", "RP", "").Replace(s)
	if s == "" {
		return 0
	}

	hasComma := strings.Contains(s, ",")
	hasDot := strings.Contains(s, ".")
	switch {
	case hasComma && hasDot:
		// Format Indonesia lengkap: titik ribuan, koma desimal.
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	case hasComma:
		s = strings.ReplaceAll(s, ",", ".")
	case hasDot && isThousandGrouped(s):
		s = strings.ReplaceAll(s, ".", "")
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

// isThousandGrouped menebak apakah titik berperan sebagai pemisah ribuan
// ("15.000") dan bukan titik desimal ("12.5"): setiap kelompok setelah yang
// pertama harus tepat tiga digit.
func isThousandGrouped(s string) bool {
	groups := strings.Split(strings.TrimPrefix(s, "-"), ".")
	if len(groups) < 2 {
		return false
	}
	for i, g := range groups {
		if g == "" {
			return false
		}
		if i > 0 && len(g) != 3 {
			return false
		}
		for _, r := range g {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

func parseInt(s string) int {
	return int(parseFloat(s))
}

// productFromRow memetakan satu baris sheet "Products" ke model domain.
func productFromRow(row []string, rowNumber int) domain.Product {
	return domain.Product{
		ID:        cell(row, 0),
		Name:      cell(row, 1),
		Category:  cell(row, 2),
		Price:     parseFloat(cell(row, 3)),
		Stock:     parseInt(cell(row, 4)),
		SKU:       cell(row, 5),
		ImageURL:  cell(row, 6),
		ImageID:   cell(row, 7),
		RowNumber: rowNumber,
	}
}

// productToRow memetakan model domain kembali ke baris sheet.
func productToRow(p domain.Product) []any {
	return []any{p.ID, p.Name, p.Category, p.Price, p.Stock, p.SKU, p.ImageURL, p.ImageID}
}

// transactionLineFromRow memetakan satu baris sheet "Transactions".
func transactionLineFromRow(row []string) domain.TransactionLine {
	return domain.TransactionLine{
		Date:          timex.Parse(cell(row, 0)),
		TransactionID: cell(row, 1),
		Item:          cell(row, 2),
		Qty:           parseInt(cell(row, 3)),
		UnitPrice:     parseFloat(cell(row, 4)),
		Total:         parseFloat(cell(row, 5)),
		PaymentMethod: strings.ToLower(cell(row, 6)),
		Cashier:       cell(row, 7),
	}
}

func transactionLineToRow(l domain.TransactionLine) []any {
	return []any{
		timex.Format(l.Date), l.TransactionID, l.Item, l.Qty,
		l.UnitPrice, l.Total, l.PaymentMethod, l.Cashier,
	}
}

// orderFromRow memetakan satu baris sheet "Orders". Daftar item disimpan
// sebagai JSON dalam satu sel agar satu pesanan tetap satu baris.
func orderFromRow(row []string, rowNumber int) domain.Order {
	var items []domain.OrderItem
	if raw := cell(row, 9); raw != "" {
		_ = json.Unmarshal([]byte(raw), &items)
	}
	return domain.Order{
		ID:            cell(row, 0),
		Code:          cell(row, 1),
		TransactionID: cell(row, 2),
		CreatedAt:     timex.Parse(cell(row, 3)),
		UpdatedAt:     timex.Parse(cell(row, 4)),
		Status:        domain.OrderStatus(strings.ToLower(cell(row, 5))),
		Source:        domain.OrderSource(strings.ToLower(cell(row, 6))),
		CustomerName:  cell(row, 7),
		TableNo:       cell(row, 8),
		Items:         items,
		Total:         parseFloat(cell(row, 10)),
		Note:          cell(row, 11),
		Cashier:       cell(row, 12),
		RowNumber:     rowNumber,
	}
}

func orderToRow(o domain.Order) []any {
	items, err := json.Marshal(o.Items)
	if err != nil {
		items = []byte("[]")
	}
	return []any{
		o.ID, o.Code, o.TransactionID, timex.Format(o.CreatedAt), timex.Format(o.UpdatedAt),
		string(o.Status), string(o.Source), o.CustomerName, o.TableNo, string(items),
		o.Total, o.Note, o.Cashier,
	}
}

// employeeFromRow memetakan satu baris sheet "Employees".
func employeeFromRow(row []string, rowNumber int) domain.Employee {
	return domain.Employee{
		ID:        cell(row, 0),
		Name:      cell(row, 1),
		Email:     strings.ToLower(cell(row, 2)),
		Role:      domain.Role(strings.ToLower(cell(row, 3))),
		Active:    !strings.EqualFold(cell(row, 4), "nonaktif"),
		PINHash:   cell(row, 5),
		CreatedAt: timex.Parse(cell(row, 6)),
		RowNumber: rowNumber,
	}
}

func employeeToRow(e domain.Employee) []any {
	status := "aktif"
	if !e.Active {
		status = "nonaktif"
	}
	return []any{e.ID, e.Name, strings.ToLower(e.Email), string(e.Role), status, e.PINHash, timex.Format(e.CreatedAt)}
}
