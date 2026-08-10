-- Mengembalikan kolom spreadsheet_id sebagai kolom kosong. Nilai lamanya
-- tidak bisa dipulihkan oleh rollback ini.
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS spreadsheet_id TEXT NOT NULL DEFAULT '';
