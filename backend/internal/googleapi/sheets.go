package googleapi

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	gapi "google.golang.org/api/googleapi"
	"google.golang.org/api/sheets/v4"
)

// SheetSpec mendeskripsikan satu sheet beserta baris headernya.
type SheetSpec struct {
	Title  string
	Header []string
}

// SheetsClient adalah pembungkus tipis Google Sheets API untuk satu
// spreadsheet milik tenant.
type SheetsClient struct {
	svc           *sheets.Service
	spreadsheetID string

	mu      sync.Mutex
	sheetID map[string]int64
}

// NewSheetsClient membuat klien untuk spreadsheet tertentu.
func NewSheetsClient(svc *sheets.Service, spreadsheetID string) *SheetsClient {
	return &SheetsClient{svc: svc, spreadsheetID: spreadsheetID, sheetID: map[string]int64{}}
}

// SpreadsheetID mengembalikan ID spreadsheet yang dikelola klien ini.
func (c *SheetsClient) SpreadsheetID() string { return c.spreadsheetID }

// EnsureSheets membuat sheet yang belum ada dan menulis header bila kosong.
// Operasi ini idempoten sehingga aman dipanggil berulang.
func (c *SheetsClient) EnsureSheets(ctx context.Context, specs []SheetSpec) error {
	meta, err := call(ctx, func() (*sheets.Spreadsheet, error) {
		return c.svc.Spreadsheets.Get(c.spreadsheetID).Context(ctx).Do()
	})
	if err != nil {
		return fmt.Errorf("membaca metadata spreadsheet: %w", err)
	}

	existing := map[string]int64{}
	for _, s := range meta.Sheets {
		if s.Properties != nil {
			existing[s.Properties.Title] = s.Properties.SheetId
		}
	}

	var requests []*sheets.Request
	for _, spec := range specs {
		if _, ok := existing[spec.Title]; !ok {
			requests = append(requests, &sheets.Request{
				AddSheet: &sheets.AddSheetRequest{
					Properties: &sheets.SheetProperties{Title: spec.Title},
				},
			})
		}
	}

	// Sheet default bernama "Sheet1" ikut dihapus bila tidak dipakai.
	if id, ok := existing["Sheet1"]; ok && !containsSpec(specs, "Sheet1") && len(existing) > 1 {
		requests = append(requests, &sheets.Request{
			DeleteSheet: &sheets.DeleteSheetRequest{SheetId: id},
		})
	}

	if len(requests) > 0 {
		if _, err := call(ctx, func() (*sheets.BatchUpdateSpreadsheetResponse, error) {
			return c.svc.Spreadsheets.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
				Requests: requests,
			}).Context(ctx).Do()
		}); err != nil {
			return fmt.Errorf("membuat sheet: %w", err)
		}
	}

	// Tulis header untuk sheet yang masih kosong.
	for _, spec := range specs {
		if len(spec.Header) == 0 {
			continue
		}
		rows, err := c.Rows(ctx, fmt.Sprintf("%s!A1:%s1", quoteSheet(spec.Title), ColumnLetter(len(spec.Header))))
		if err != nil {
			return err
		}
		if len(rows) > 0 && len(rows[0]) > 0 {
			continue
		}
		header := make([]any, len(spec.Header))
		for i, h := range spec.Header {
			header[i] = h
		}
		if err := c.UpdateRow(ctx, spec.Title, 1, header); err != nil {
			return fmt.Errorf("menulis header %s: %w", spec.Title, err)
		}
	}

	c.mu.Lock()
	c.sheetID = map[string]int64{}
	c.mu.Unlock()
	return nil
}

func containsSpec(specs []SheetSpec, title string) bool {
	for _, s := range specs {
		if s.Title == title {
			return true
		}
	}
	return false
}

// Rows membaca rentang A1 dan mengembalikannya sebagai matriks string.
func (c *SheetsClient) Rows(ctx context.Context, rangeA1 string) ([][]string, error) {
	resp, err := call(ctx, func() (*sheets.ValueRange, error) {
		return c.svc.Spreadsheets.Values.Get(c.spreadsheetID, rangeA1).
			ValueRenderOption("UNFORMATTED_VALUE").
			DateTimeRenderOption("FORMATTED_STRING").
			Context(ctx).Do()
	})
	if err != nil {
		return nil, fmt.Errorf("membaca rentang %s: %w", rangeA1, err)
	}
	out := make([][]string, 0, len(resp.Values))
	for _, row := range resp.Values {
		cells := make([]string, len(row))
		for i, cell := range row {
			cells[i] = toString(cell)
		}
		out = append(out, cells)
	}
	return out, nil
}

// SheetRows membaca seluruh baris data (tanpa header) dari sebuah sheet
// beserta nomor baris aslinya. Nomor baris dipakai untuk update/hapus.
func (c *SheetsClient) SheetRows(ctx context.Context, title string, cols int) ([][]string, error) {
	rangeA1 := fmt.Sprintf("%s!A2:%s", quoteSheet(title), ColumnLetter(cols))
	return c.Rows(ctx, rangeA1)
}

// Append menambahkan satu atau beberapa baris di akhir sheet.
func (c *SheetsClient) Append(ctx context.Context, title string, rows [][]any) error {
	if len(rows) == 0 {
		return nil
	}
	_, err := call(ctx, func() (*sheets.AppendValuesResponse, error) {
		return c.svc.Spreadsheets.Values.Append(c.spreadsheetID, quoteSheet(title)+"!A1", &sheets.ValueRange{
			Values: rows,
		}).ValueInputOption("USER_ENTERED").InsertDataOption("INSERT_ROWS").Context(ctx).Do()
	})
	if err != nil {
		return fmt.Errorf("menambah baris ke %s: %w", title, err)
	}
	return nil
}

// UpdateRow menimpa satu baris penuh (1-based, termasuk header).
func (c *SheetsClient) UpdateRow(ctx context.Context, title string, rowNumber int, row []any) error {
	if rowNumber < 1 {
		return errors.New("nomor baris tidak valid")
	}
	rangeA1 := fmt.Sprintf("%s!A%d:%s%d", quoteSheet(title), rowNumber, ColumnLetter(len(row)), rowNumber)
	_, err := call(ctx, func() (*sheets.UpdateValuesResponse, error) {
		return c.svc.Spreadsheets.Values.Update(c.spreadsheetID, rangeA1, &sheets.ValueRange{
			Values: [][]any{row},
		}).ValueInputOption("USER_ENTERED").Context(ctx).Do()
	})
	if err != nil {
		return fmt.Errorf("memperbarui baris %d pada %s: %w", rowNumber, title, err)
	}
	return nil
}

// CellUpdate adalah satu perubahan sel/rentang untuk operasi batch.
type CellUpdate struct {
	Range  string
	Values []any
}

// BatchUpdate menerapkan beberapa perubahan rentang dalam satu panggilan API.
func (c *SheetsClient) BatchUpdate(ctx context.Context, updates []CellUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	data := make([]*sheets.ValueRange, 0, len(updates))
	for _, u := range updates {
		data = append(data, &sheets.ValueRange{Range: u.Range, Values: [][]any{u.Values}})
	}
	_, err := call(ctx, func() (*sheets.BatchUpdateValuesResponse, error) {
		return c.svc.Spreadsheets.Values.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateValuesRequest{
			ValueInputOption: "USER_ENTERED",
			Data:             data,
		}).Context(ctx).Do()
	})
	if err != nil {
		return fmt.Errorf("batch update gagal: %w", err)
	}
	return nil
}

// DeleteRow menghapus satu baris beserta menggeser baris di bawahnya.
func (c *SheetsClient) DeleteRow(ctx context.Context, title string, rowNumber int) error {
	id, err := c.sheetIDByTitle(ctx, title)
	if err != nil {
		return err
	}
	_, err = call(ctx, func() (*sheets.BatchUpdateSpreadsheetResponse, error) {
		return c.svc.Spreadsheets.BatchUpdate(c.spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
			Requests: []*sheets.Request{{
				DeleteDimension: &sheets.DeleteDimensionRequest{
					Range: &sheets.DimensionRange{
						SheetId:    id,
						Dimension:  "ROWS",
						StartIndex: int64(rowNumber - 1),
						EndIndex:   int64(rowNumber),
					},
				},
			}},
		}).Context(ctx).Do()
	})
	if err != nil {
		return fmt.Errorf("menghapus baris %d pada %s: %w", rowNumber, title, err)
	}
	return nil
}

func (c *SheetsClient) sheetIDByTitle(ctx context.Context, title string) (int64, error) {
	c.mu.Lock()
	if id, ok := c.sheetID[title]; ok {
		c.mu.Unlock()
		return id, nil
	}
	c.mu.Unlock()

	meta, err := call(ctx, func() (*sheets.Spreadsheet, error) {
		return c.svc.Spreadsheets.Get(c.spreadsheetID).Context(ctx).Do()
	})
	if err != nil {
		return 0, fmt.Errorf("membaca metadata spreadsheet: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range meta.Sheets {
		if s.Properties != nil {
			c.sheetID[s.Properties.Title] = s.Properties.SheetId
		}
	}
	id, ok := c.sheetID[title]
	if !ok {
		return 0, fmt.Errorf("sheet %q tidak ditemukan", title)
	}
	return id, nil
}

// ColumnLetter mengubah indeks kolom 1-based menjadi label A1 (1 -> A, 27 -> AA).
func ColumnLetter(n int) string {
	if n < 1 {
		n = 1
	}
	var sb strings.Builder
	for n > 0 {
		n--
		sb.WriteByte(byte('A' + n%26))
		n /= 26
	}
	runes := []byte(sb.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// quoteSheet membungkus nama sheet dengan tanda kutip tunggal agar aman
// dipakai dalam notasi A1 meski mengandung spasi.
func quoteSheet(title string) string {
	return "'" + strings.ReplaceAll(title, "'", "''") + "'"
}

func toString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return trimFloat(t)
	case bool:
		if t {
			return "TRUE"
		}
		return "FALSE"
	default:
		return fmt.Sprintf("%v", t)
	}
}

func trimFloat(f float64) string {
	s := fmt.Sprintf("%.10f", f)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// IsNotFound memeriksa apakah error berasal dari Google API dengan status 404.
func IsNotFound(err error) bool {
	var gerr *gapi.Error
	return errors.As(err, &gerr) && gerr.Code == 404
}
