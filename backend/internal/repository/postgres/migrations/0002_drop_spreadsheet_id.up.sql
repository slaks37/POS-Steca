-- Google Sheets tidak lagi menjadi datastore: seluruh data terstruktur berada
-- di PostgreSQL, dan Google Drive hanya dipakai untuk gambar produk. Kolom
-- spreadsheet_id karena itu sudah tidak punya arti dan dibuang agar tidak ada
-- sisa data yang membingungkan.
--
-- folder_id TETAP dipakai: itu folder Drive tempat gambar produk disimpan.
ALTER TABLE tenants DROP COLUMN IF EXISTS spreadsheet_id;
