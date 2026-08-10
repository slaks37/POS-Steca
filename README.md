# Steca POS

Aplikasi **Point of Sale untuk UMKM Indonesia** — terinspirasi kombinasi Majoo, Moka POS,
dan modul manajemen pesanan F&B ala Trofi.

Data produksi tersimpan di **PostgreSQL** dengan skema multi-tenant, sementara **Google
Drive** milik tiap pemilik usaha tetap dipakai untuk login OAuth dan menyimpan gambar produk.
Arsitekturnya berlapis port/repository, jadi datastore bisa diganti tanpa menyentuh logika bisnis.

```
┌──────────────┐   REST/JSON    ┌──────────────┐   SQL (tenant_id)   ┌────────────────┐
│  React SPA   │ ─────────────► │  Backend Go  │ ──────────────────► │  PostgreSQL    │
│  + Android   │ ◄───────────── │  Gin, modular│ ◄────────────────── │  (multi-tenant)│
└──────────────┘                └──────┬───────┘                     └────────────────┘
                                       │  OAuth2 per tenant
                                       ▼
                               ┌────────────────┐
                               │  Google Drive  │  login pemilik + gambar produk
                               └────────────────┘
```

> **Catatan riwayat.** Versi awal aplikasi ini memakai Google Sheets sebagai sumber
> kebenaran. Perannya sudah digantikan sepenuhnya oleh PostgreSQL, dan seluruh kode
> datastore Sheets telah dihapus dari repositori. Google kini hanya dipakai untuk login
> OAuth pemilik dan penyimpanan gambar produk di Drive.

---

## Daftar isi

- [Fitur](#fitur)
- [Tech stack](#tech-stack)
- [Penyimpanan data](#penyimpanan-data)
- [Struktur project](#struktur-project)
- [Menjalankan cepat (mode demo, tanpa akun Google)](#menjalankan-cepat-mode-demo-tanpa-akun-google)
- [Setup Google Cloud (mode produksi)](#setup-google-cloud-mode-produksi)
- [Menjalankan mode produksi](#menjalankan-mode-produksi)
- [PostgreSQL (datastore produksi)](#postgresql-datastore-produksi)
- [Referensi REST API](#referensi-rest-api)
- [Hak akses per role](#hak-akses-per-role)
- [Aplikasi Android (Capacitor)](#aplikasi-android-capacitor)
- [Pengujian](#pengujian)
- [Catatan desain dan batasan](#catatan-desain-dan-batasan)

---

## Fitur

| Modul | Ringkasan |
|---|---|
| **Autentikasi & multi-tenant** | Pemilik masuk lewat Google OAuth2; setiap bisnis terhubung ke akun Google-nya sendiri. Karyawan masuk dengan kode bisnis + email + PIN. |
| **Kasir** | Pilih produk, keranjang, hitung kembalian, metode tunai/QRIS/kartu, struk digital siap cetak. |
| **Manajemen pesanan (ala Trofi)** | Papan `baru → diproses → selesai`, sumber pesanan `kasir langsung` atau `online`, pembatalan mengembalikan stok. |
| **Inventori/produk** | CRUD produk beserta stok, termasuk unggah foto produk ke folder Drive tenant. |
| **Laporan penjualan** | Dibaca dari riwayat transaksi: harian/mingguan/bulanan, produk terlaris, total omzet, metode pembayaran, performa kasir. |
| **CRM & loyalitas** | Basis pelanggan per tenant. Kasir boleh menautkan pelanggan saat checkout — opsional, transaksi anonim tetap sah. Poin terkumpul otomatis 1 poin per Rp 10.000 belanja, dan owner punya halaman pelanggan lengkap dengan riwayat pembelian. |
| **Denah / nomor meja** | Nomor/nama meja beserta kapasitasnya. Status `kosong / terisi / perlu dibersihkan` terlihat di layar kasir dan papan pesanan, dan setiap transaksi bisa dikaitkan ke meja tertentu. |
| **Karyawan** | Role `owner` dan `kasir` dengan level akses berbeda, PIN di-hash bcrypt. |
| **Dashboard owner** | Omzet hari ini & bulan ini, tren 7 hari, produk terlaris, pesanan berjalan, peringatan stok menipis. |
| **Kanal pesan online** | Halaman menu publik `/menu/<KODE-BISNIS>` tanpa login; pesanan langsung masuk ke papan pesanan kasir. |

---

## Tech stack

- **Backend** — Go 1.24, [Gin](https://github.com/gin-gonic/gin), REST API, struktur modular
  (handler → service → repository), JWT HS256, bcrypt untuk PIN.
- **Frontend** — React 18 + Vite + TypeScript (strict), React Router, tanpa dependensi UI
  eksternal (CSS ditulis sendiri di atas design token agar bundel tetap ringan).
  Tata letak dioptimalkan untuk tablet kasir: sidebar menyusut jadi rel ikon di bawah
  1320px dan target sentuh diperbesar pada layar ≤900px.
- **Datastore** — PostgreSQL 14+ lewat [pgx](https://github.com/jackc/pgx) dengan skema
  multi-tenant dan migrasi SQL terembed. Google Drive API v3 dipakai untuk OAuth2 per akun
  pengguna dan penyimpanan gambar produk.
- **Android** — [Capacitor](https://capacitorjs.com/) membungkus build web yang sama menjadi
  APK/AAB (`id.stecapos.app`), tanpa menulis ulang UI. Lihat
  [Aplikasi Android](#aplikasi-android-capacitor).

---

## Penyimpanan data

Ada dua tempat penyimpanan, dengan pembagian tugas yang tegas:

| Apa | Di mana | Kenapa |
|---|---|---|
| Data terstruktur — tenant, karyawan, produk, transaksi, pesanan, pelanggan, meja | **PostgreSQL** | satu sumber kebenaran, terisolasi per tenant, dijaga constraint database |
| Berkas biner — gambar produk | **Google Drive milik pemilik akun** | file tetap milik pemilik usaha dan bisa dibuka langsung |
| Identitas pemilik | **Google OAuth2** | pemilik masuk dengan akun Google-nya sendiri |

Saat pemilik pertama kali masuk dengan Google, backend membuat satu folder Drive khusus
tenant tersebut:

```
Drive pemilik akun/
└── Steca POS - <Nama Bisnis>/          ← folder khusus tenant
    ├── produk-20260809-150405-a1b2c.jpg
    └── produk-...                       ← seluruh foto produk tenant
```

Tidak ada spreadsheet yang dibuat: seluruh data terstruktur langsung masuk ke PostgreSQL.
Tautan folder ini bisa dibuka pemilik dari halaman **Pengaturan**.

Refresh token Google **selalu terenkripsi AES-256-GCM** memakai `ENCRYPTION_KEY` sebelum
disimpan pada kolom `tenants.refresh_token_enc`, tidak pernah tersimpan sebagai teks biasa,
dan tidak pernah dikirim ke frontend.

Skema tabelnya dijelaskan di [PostgreSQL (datastore produksi)](#postgresql-datastore-produksi).

---

## Struktur project

```
backend/
├── cmd/server/main.go              # entrypoint + wiring dependensi
└── internal/
    ├── config/                     # pemuatan & validasi env
    ├── domain/                     # model inti + kontrak repository (port)
    ├── apperr/                     # error aplikasi → status HTTP
    ├── crypto/                     # AES-256-GCM untuk refresh token
    ├── timex/                      # zona waktu & format tanggal (WIB default)
    ├── googleapi/                  # pembungkus OAuth2 + Google Drive
    ├── repository/
    │   ├── postgres/               # implementasi port di atas PostgreSQL (produksi)
    │   │   └── migrations/         # SQL terurut, di-embed ke biner
    │   ├── drivestore/             # folder & gambar produk di Google Drive
    │   └── memory/                 # driver in-memory (demo & pengembangan)
    ├── service/                    # aturan bisnis: auth, kasir, pesanan, laporan…
    └── handler/                    # routing Gin, middleware, RBAC, validasi

docker-compose.yml                  # PostgreSQL lokal untuk development

frontend/
├── index.html
├── capacitor.config.ts             # appId id.stecapos.app, webDir dist
├── assets/                         # sumber ikon & splash (1024px / 2732px)
├── android/                        # project Gradle hasil `cap add android`
└── src/
    ├── components/                 # Layout, komponen UI, struk, pemilih pelanggan
    ├── context/AuthContext.tsx     # state sesi
    ├── lib/                        # klien API, tipe, format rupiah/tanggal,
    │                               # penyesuaian native (splash & status bar)
    ├── styles.css                  # design token: warna, tipografi, jarak, radius
    └── pages/                      # Login, Dashboard, Kasir, Pesanan, Meja, Produk,
                                    # Pelanggan, Laporan, Karyawan, Pengaturan,
                                    # dan halaman menu publik
```

Alur dependensi selalu satu arah: `handler → service → domain (port) ← repository`.
Layer service tidak pernah menyentuh Gin maupun SDK Google, sehingga bisa diuji tanpa jaringan.

---

## Menjalankan cepat (mode demo, tanpa akun Google)

Cara tercepat mencoba seluruh alur aplikasi. Datastore memakai memori proses, jadi tidak
perlu kredensial Google apa pun.

**Terminal 1 — backend:**

```bash
cd backend
POS_DATASTORE=memory go run ./cmd/server
```

**Terminal 2 — frontend:**

```bash
cd frontend
npm install
npm run dev
```

Buka <http://localhost:5173>, lalu klik **"Masuk mode demo"**. Tenant contoh dibuat berisi
10 produk, akun pemilik, dan akun kasir:

| Akun | Email | PIN |
|---|---|---|
| Pemilik | `owner@demo.local` | `112233` |
| Kasir | `kasir@demo.local` | `123456` |

Kode bisnis untuk login karyawan ditampilkan di halaman login setelah demo dibuat, dan juga
di sidebar serta halaman **Pengaturan**.

---

## Setup Google Cloud (mode produksi)

1. Buka [Google Cloud Console](https://console.cloud.google.com/) → buat project baru.
2. **APIs & Services → Library** → aktifkan **Google Drive API**.
3. **APIs & Services → OAuth consent screen**:
   - User type: *External* (atau *Internal* bila memakai Google Workspace).
   - Tambahkan scope: `.../auth/drive.file`, `openid`, `.../auth/userinfo.email`,
     `.../auth/userinfo.profile`.
   - Selama masih berstatus *Testing*, tambahkan email pemilik usaha sebagai **Test users**.
4. **APIs & Services → Credentials → Create Credentials → OAuth client ID**:
   - Application type: **Web application**.
   - Authorized redirect URIs: `http://localhost:8080/api/v1/auth/google/callback`
     (dan URL produksi Anda, misal `https://api.tokoanda.com/api/v1/auth/google/callback`).
5. Salin **Client ID** dan **Client secret** ke `backend/.env`.

> Scope `drive.file` sengaja dipilih alih-alih `drive` penuh: aplikasi hanya bisa melihat
> dan mengubah berkas yang **dibuatnya sendiri**, bukan seluruh isi Drive pengguna.

---

## Menjalankan mode produksi

```bash
cd backend
cp .env.example .env
# isi POS_DATASTORE=postgres, DATABASE_URL, GOOGLE_CLIENT_ID,
# GOOGLE_CLIENT_SECRET, JWT_SECRET, ENCRYPTION_KEY
# (JWT_SECRET dan ENCRYPTION_KEY: openssl rand -base64 48)

set -a && source .env && set +a
go run ./cmd/server     # migrasi skema berjalan otomatis saat start
```

Database produksi disiapkan sendiri oleh pemilik aplikasi — lihat
[PostgreSQL (datastore produksi)](#postgresql-datastore-produksi).

```bash
cd frontend
npm install
npm run build        # hasil di frontend/dist, layani dengan Nginx/Caddy/hosting statis
```

Frontend memakai routing sisi klien, jadi web server statis perlu **fallback ke
`index.html`** agar URL seperti `/kasir` atau `/menu/STC-ABCDE` tidak menghasilkan 404.
Contoh Nginx:

```nginx
location / {
    try_files $uri $uri/ /index.html;
}
```

Bila frontend dan backend berbeda domain, set `VITE_API_BASE_URL` ke URL penuh backend
(misal `https://api.tokoanda.com/api/v1`) dan pastikan domain frontend terdaftar pada
`CORS_ORIGINS` di backend.

Backend akan menolak start bila `DATABASE_URL`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`,
`JWT_SECRET`, atau `ENCRYPTION_KEY` belum diisi pada mode `postgres` — supaya salah
konfigurasi ketahuan saat deploy, bukan saat pengguna pertama login.

Seluruh variabel env terdokumentasi di [`backend/.env.example`](backend/.env.example).

---

## PostgreSQL (datastore produksi)

PostgreSQL adalah satu-satunya sumber kebenaran data produksi. Datastore Google Sheets yang
dipakai versi awal sudah dihapus seluruhnya agar tidak ada dua jalur logika yang harus
dijaga selaras.

Arsitektur port/repository tidak berubah: layer service tetap berbicara ke antarmuka di
`internal/domain`, dan hanya implementasinya yang berganti.

```
handler → service → domain (port) ← implementasi:
                                     ├── repository/postgres   ← produksi
                                     └── repository/memory     ← demo & pengembangan

                                     repository/drivestore     ← folder & gambar produk (Drive)
```

### Mode datastore

| `POS_DATASTORE` | Sumber kebenaran | Kebutuhan |
|---|---|---|
| `postgres` | PostgreSQL | `DATABASE_URL`, kredensial Google (login pemilik + gambar produk) |
| `memory` | RAM proses | tanpa setup apa pun — dipakai mode demo |

Hanya dua mode ini yang ada: `postgres` untuk produksi, `memory` untuk demo dan pengujian
tanpa infrastruktur. Pada mode `postgres`, Google Drive tetap dipakai untuk **dua hal**:
OAuth login pemilik dan penyimpanan gambar produk — karena itu saat onboarding hanya folder
Drive yang disiapkan.

### Skema multi-tenant

Setiap tabel entitas memakai kunci utama gabungan `(tenant_id, id)` dan foreign key ke
`tenants` dengan `ON DELETE CASCADE`. **Semua query di-scope `tenant_id = $1`**, sehingga
data antar bisnis tidak bisa saling terbaca meskipun ID entitasnya kebetulan sama.

| Tabel | Isi |
|---|---|
| `tenants` | Identitas bisnis + folder Drive-nya. Refresh token Google **terenkripsi AES-256-GCM** oleh `internal/crypto`; database hanya menerima ciphertext. |
| `employees` | Karyawan + PIN bcrypt. Unik per tenant berdasarkan email. |
| `products` | Katalog. SKU unik per tenant (boleh kosong). |
| `customers` | CRM & loyalitas. `phone_normalized` diisi aplikasi memakai `domain.NormalizePhone`, unik per tenant. |
| `tables` | Denah meja. Nama unik per tenant, status dijaga `CHECK`. |
| `orders` | Pesanan; daftar item disimpan sebagai `JSONB` agar tetap satu baris per pesanan. |
| `transaction_lines` | Append-only, satu baris per item terjual; dasar seluruh laporan penjualan. |

Aturan yang dulu hanya dijaga aplikasi kini juga dijaga database: `CHECK` untuk status
pesanan/meja dan role, indeks unik untuk SKU/email/nomor HP, serta `stock >= 0`.

### Migrasi

Migrasi berupa berkas SQL terurut di `backend/internal/repository/postgres/migrations/`,
dengan penamaan `<versi>_<nama>.up.sql` dan `.down.sql`:

```
0001_init.up.sql
0001_init.down.sql
0002_drop_spreadsheet_id.up.sql
0002_drop_spreadsheet_id.down.sql
```

Berkas tersebut **di-embed ke dalam biner** (`go:embed`), jadi deployment cukup mengirim satu
executable. Saat aplikasi start, migrasi yang belum pernah dijalankan diterapkan otomatis:

- versi yang sudah dijalankan dicatat di tabel `schema_migrations`, jadi aman dipanggil berulang;
- setiap migrasi berjalan dalam **satu transaksi**, sehingga tidak ada skema setengah jadi;
- proses dilindungi **advisory lock**, jadi dua instance yang start bersamaan tidak bentrok.

Setel `RUN_MIGRATIONS=false` bila Anda menjalankan migrasi terpisah di pipeline deployment.

Menambah perubahan skema berikutnya: buat `0003_<nama>.up.sql` dan `.down.sql`, lalu jalankan
aplikasi seperti biasa. Migrasi `0002` adalah contohnya — kolom `tenants.spreadsheet_id`
dibuang setelah datastore Sheets dihapus.

### PostgreSQL lokal untuk development

Gunakan `docker-compose.yml` di root repositori:

```bash
docker compose up -d              # jalankan PostgreSQL 16 di localhost:5432
docker compose ps                 # pastikan healthy
docker compose --profile tools up -d   # opsional: Adminer di http://localhost:8081
docker compose down               # hentikan (data tetap tersimpan di volume)
docker compose down -v            # hentikan sekaligus hapus seluruh data
```

Lalu jalankan backend:

```bash
cd backend
cp .env.example .env
# isi minimal: DATABASE_URL, JWT_SECRET, ENCRYPTION_KEY,
#              GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET

set -a && source .env && set +a
go run ./cmd/server
```

Tabel dibuat otomatis pada start pertama. Untuk mengintip isinya:

```bash
psql "postgres://pos:pos@localhost:5432/pos_steca" -c "\dt"
```

> Ingin mencoba aplikasi tanpa menyiapkan apa pun? Pakai `POS_DATASTORE=memory` dan tombol
> **"Masuk mode demo"** — lihat [Menjalankan cepat](#menjalankan-cepat-mode-demo-tanpa-akun-google).

### PostgreSQL untuk produksi

**Penyediaan database produksi adalah tanggung jawab pemilik aplikasi.** Repositori ini tidak
bisa (dan tidak seharusnya) menyediakannya, karena butuh akun, tagihan, dan kredensial pribadi
di luar kendali lingkungan pengembangan.

Anda bebas memilih:

- **Managed** — Neon, Supabase, Railway, Render, Amazon RDS, Google Cloud SQL, Azure Database
  for PostgreSQL, DigitalOcean Managed Databases, Aiven, dan sejenisnya.
- **Self-hosted** — PostgreSQL di VPS sendiri, atau container di server Anda.

Yang dibutuhkan aplikasi hanyalah `DATABASE_URL`. Beberapa hal yang perlu Anda pastikan:

- **PostgreSQL 14 atau lebih baru** (dikembangkan dan diuji dengan versi 16).
- **SSL aktif** untuk koneksi lewat internet: `?sslmode=require` pada URL.
- **Backup rutin dan teruji.** Database adalah satu-satunya salinan data penjualan; tidak
  ada cadangan otomatis di tempat lain.
- **Kuota koneksi.** Pool dibatasi 10 koneksi per instance aplikasi; sesuaikan bila paket
  provider Anda lebih kecil.
- **`ENCRYPTION_KEY` yang sama** dengan yang dipakai sebelumnya. Kunci ini membuka
  `tenants.refresh_token_enc`; menggantinya membuat semua pemilik harus menghubungkan ulang
  akun Google.

Contoh `DATABASE_URL` produksi:

```
postgres://pengguna:sandi@host-penyedia.example:5432/pos_steca?sslmode=require
```

### Instalasi lama berbasis Google Sheets

Mode `POS_DATASTORE=google` sudah **tidak ada lagi**: backend akan menolak start bila nilai
itu masih tersetel, dan pesan galatnya menyebutkan pilihan yang tersedia. Instalasi lama
perlu menyiapkan PostgreSQL lalu memasukkan datanya sendiri — spreadsheet tenant tetap utuh
di Drive pemilik dan bisa diekspor sebagai CSV untuk keperluan itu.

---

## Referensi REST API

Base path: `/api/v1`. Semua respons memakai amplop `{"data": ...}`, dan galat memakai
`{"error": {"code": "...", "message": "..."}}` dengan pesan berbahasa Indonesia.

### Tanpa autentikasi

| Method | Endpoint | Keterangan |
|---|---|---|
| `GET` | `/health` | Status server dan mode datastore. |
| `GET` | `/auth/google/url` | URL consent Google untuk onboarding pemilik. |
| `GET` | `/auth/google/callback` | Callback OAuth; redirect ke frontend dengan token pada fragment URL. |
| `POST` | `/auth/staff/login` | Login karyawan: `{tenant_code, email, pin}`. |
| `POST` | `/auth/demo/login` | Hanya aktif pada `POS_DATASTORE=memory`. |
| `GET` | `/public/:code/menu` | Menu publik satu bisnis. |
| `POST` | `/public/:code/orders` | Pesanan online dari pelanggan. |

### Perlu `Authorization: Bearer <token>`

| Method | Endpoint | Role | Keterangan |
|---|---|---|---|
| `GET` | `/auth/me` | semua | Profil pengguna + tenant aktif. |
| `PATCH` | `/tenant` | owner | Ubah nama bisnis. |
| `GET` | `/products` | semua | Katalog produk (`?q=`, `?category=`). |
| `GET` | `/products/categories` | semua | Daftar kategori unik. |
| `GET` | `/products/:id` | semua | Detail produk. |
| `POST` | `/products` | owner | Tambah produk. |
| `PUT` | `/products/:id` | owner | Ubah produk. |
| `DELETE` | `/products/:id` | owner | Hapus produk + gambarnya di Drive. |
| `POST` | `/product-images` | owner | Unggah gambar (multipart, field `file`, maks 5 MB). |
| `POST` | `/checkout` | semua | Transaksi kasir; mengembalikan pesanan + struk. |
| `GET` | `/orders` | semua | Daftar pesanan (`?status=`, `?source=`, `?from=`, `?to=`). |
| `GET` | `/orders/:id` | semua | Detail pesanan. |
| `PATCH` | `/orders/:id/status` | semua | Ubah status pesanan. |
| `POST` | `/orders/:id/settle` | semua | Terima pembayaran pesanan online. |
| `GET` | `/dashboard` | owner | Ringkasan performa toko. |
| `GET` | `/reports/sales` | owner | Laporan penjualan (`?granularity=harian\|mingguan\|bulanan`). |
| `GET` | `/reports/transactions` | owner | Riwayat struk. |
| `GET` `POST` `PUT` `DELETE` | `/employees[/:id]` | owner | Manajemen karyawan. |
| `GET` | `/customers` | semua | Daftar/pencarian pelanggan (`?q=`). |
| `GET` | `/customers/:id` | semua | Detail pelanggan. |
| `POST` | `/customers` | semua | Daftarkan pelanggan (kasir boleh, saat transaksi). |
| `PUT` `DELETE` | `/customers/:id` | owner | Ubah / hapus pelanggan. |
| `GET` | `/customers/:id/history` | owner | Riwayat pembelian pelanggan. |
| `GET` | `/tables` | semua | Denah meja beserta statusnya. |
| `PATCH` | `/tables/:id/status` | semua | Ubah status meja. |
| `POST` `PUT` `DELETE` | `/tables[/:id]` | owner | Kelola daftar meja. |

Contoh checkout:

```bash
curl -X POST http://localhost:8080/api/v1/checkout \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{
        "items": [{"product_id": "PRD-XXXX", "qty": 2}],
        "payment_method": "tunai",
        "amount_paid": 100000,
        "customer_name": "Budi",
        "customer_phone": "081234567890",
        "table_id": "TBL-XXXXXX"
      }'
```

---

## Hak akses per role

| Modul | Owner | Kasir |
|---|:---:|:---:|
| Dashboard | ✅ | — |
| Kasir & checkout | ✅ | ✅ |
| Papan pesanan | ✅ | ✅ |
| Lihat katalog produk | ✅ | ✅ |
| Tambah/ubah/hapus produk | ✅ | — |
| Lihat denah meja & ubah statusnya | ✅ | ✅ |
| Tambah/ubah/hapus meja | ✅ | — |
| Cari & daftarkan pelanggan | ✅ | ✅ |
| Riwayat pelanggan, ubah & hapus | ✅ | — |
| Laporan penjualan | ✅ | — |
| Manajemen karyawan | ✅ | — |
| Pengaturan bisnis | ✅ | baca saja |

Pembatasan diberlakukan di backend (middleware `RequireRole`), bukan hanya disembunyikan
di UI. Akun pemilik Google tidak bisa dihapus, dinonaktifkan, atau diturunkan rolenya.

---

## Aplikasi Android (Capacitor)

Frontend React yang sama dibungkus menjadi aplikasi Android memakai
[Capacitor](https://capacitorjs.com/) — **UI tidak ditulis ulang**. WebView memuat hasil
`npm run build`, dan project Gradle standar tersedia di `frontend/android/`.

| Item | Nilai |
|---|---|
| Application ID | `id.stecapos.app` |
| Nama aplikasi | Steca POS |
| minSdk / targetSdk | 24 / 36 |
| Izin | `INTERNET` saja |

### Alamat backend wajib berupa URL penuh

WebView Android berjalan pada origin `https://localhost`, sehingga **`localhost` di dalam
aplikasi menunjuk ke ponsel itu sendiri**, bukan ke komputer pengembang. Build Android
karena itu wajib menyetel `VITE_API_BASE_URL` ke URL penuh backend:

```bash
cd frontend
cp .env.android.example .env.android
# isi VITE_API_BASE_URL, contoh:
#   VITE_API_BASE_URL=https://api.tokoanda.com/api/v1
```

Backend juga perlu mengizinkan origin WebView pada CORS:

```bash
CORS_ORIGINS=https://pos.tokoanda.com,https://localhost
```

> Untuk uji coba di jaringan lokal, pakai alamat IP komputer pengembang
> (`http://192.168.1.10:8080/api/v1`), bukan `localhost`. Android 9+ memblokir HTTP polos,
> jadi untuk rilis sungguhan gunakan HTTPS.

### Menyiapkan lingkungan build

Dibutuhkan **JDK 17+** dan **Android SDK** (paling praktis lewat Android Studio; atau
`cmdline-tools` + `platform-tools` + `platforms;android-36` + `build-tools;36.0.0`).
Beri tahu Gradle lokasi SDK lewat salah satu cara:

```bash
export ANDROID_HOME="$HOME/Android/Sdk"
# atau buat frontend/android/local.properties berisi:
#   sdk.dir=/home/nama-anda/Android/Sdk
```

### Membangun ulang aset web ke project Android

Setiap kali kode frontend berubah, jalankan:

```bash
cd frontend
npm install
npm run android:sync     # = build:android + cap sync android
```

`cap sync` menyalin `dist/` ke `android/app/src/main/assets/public/` dan memperbarui daftar
plugin. Folder hasil salinan sengaja tidak ikut di-commit, jadi setelah clone baru perintah
di atas wajib dijalankan sebelum build Gradle.

Untuk membuka di Android Studio: `npm run android:open`.

### Ikon dan splash screen

Sumber gambar ada di `frontend/assets/` (`icon-only.png`, `icon-foreground.png`,
`icon-background.png`, `splash.png`, `splash-dark.png`) memakai warna merek
`#0f9384` / `#073b36`. Setelah menggantinya, buat ulang seluruh densitas dengan:

```bash
npm run android:assets
```

### Build APK (untuk uji coba / bagi manual)

APK debug tidak perlu keystore dan langsung bisa dipasang ke perangkat:

```bash
cd frontend/android
./gradlew assembleDebug
# hasil: app/build/outputs/apk/debug/app-debug.apk
```

Pasang ke perangkat yang tersambung: `adb install -r app/build/outputs/apk/debug/app-debug.apk`.

### Membuat signing key

Play Store hanya menerima build yang ditandatangani. Buat keystore **satu kali** dan simpan
baik-baik — kehilangan keystore berarti Anda tidak bisa lagi merilis pembaruan untuk
aplikasi yang sama.

```bash
cd frontend/android
keytool -genkey -v \
  -keystore steca-pos-release.jks \
  -keyalg RSA -keysize 2048 -validity 10000 \
  -alias steca-pos
```

`keytool` akan menanyakan password keystore, nama, organisasi, dan kota. Selanjutnya daftarkan
kredensialnya:

```bash
cp keystore.properties.example keystore.properties
# isi storeFile, storePassword, keyAlias, keyPassword
```

`android/app/build.gradle` membaca berkas tersebut dan otomatis menandatangani build rilis.
Bila `keystore.properties` tidak ada, build tetap berjalan tetapi hasilnya tidak
ditandatangani. Berkas `*.jks`, `*.keystore`, dan `keystore.properties` sudah masuk
`.gitignore` — **jangan pernah commit keystore atau passwordnya**.

### Build release APK dan AAB

```bash
cd frontend/android

# APK rilis (untuk distribusi manual / di luar Play Store)
./gradlew assembleRelease
# hasil: app/build/outputs/apk/release/app-release.apk

# Android App Bundle (format yang diminta Play Store)
./gradlew bundleRelease
# hasil: app/build/outputs/bundle/release/app-release.aab
```

Verifikasi tanda tangan sebelum diunggah:

```bash
$ANDROID_HOME/build-tools/36.0.0/apksigner verify --print-certs \
  app/build/outputs/apk/release/app-release.apk
```

### Menaikkan versi setiap rilis

Play Store menolak unggahan dengan `versionCode` yang sama. Sebelum build rilis berikutnya,
naikkan nilainya di `frontend/android/app/build.gradle`:

```gradle
versionCode 2          // wajib naik setiap unggahan
versionName "1.1.0"    // versi yang dilihat pengguna
```

### Login pemilik di aplikasi Android

Google memblokir alur OAuth di dalam WebView aplikasi (galat `disallowed_useragent`), jadi
tombol "Masuk dengan Google" sengaja tidak ditampilkan pada APK. Alurnya:

1. Pemilik menghubungkan akun Google **sekali** lewat peramban di versi web Steca POS.
2. Di menu **Karyawan**, pemilik mengatur PIN untuk akunnya sendiri.
3. Di aplikasi Android, masuk memakai **kode bisnis + email + PIN** seperti kasir.

Kasir sendiri memang sudah memakai jalur PIN, sehingga tidak terpengaruh.

### Yang menjadi tanggung jawab pemilik aplikasi

Repositori ini hanya menyiapkan project sampai menghasilkan APK/AAB. Langkah berikut
**tidak bisa diotomatiskan** karena butuh akun, identitas, dan kredensial pribadi:

- **Membuat akun Google Play Console** (biaya pendaftaran satu kali dari Google) serta
  verifikasi identitas/alamat pengembang.
- **Membuat aplikasi baru di Play Console** dan mengunggah `app-release.aab`.
- **Mengisi listing toko**: judul, deskripsi, tangkapan layar, ikon toko 512×512, banner
  1024×500, kategori, dan kontak.
- **Kebijakan privasi** yang bisa diakses publik — wajib, terlebih karena aplikasi ini
  mengakses Google Drive milik pengguna.
- **Data safety form**, target audiens, serta deklarasi izin bila nanti `CAMERA` diaktifkan.
- **Play App Signing**: Google menyarankan menyerahkan kunci penandatanganan aplikasi ke
  Google; kunci upload yang Anda buat di atas tetap milik Anda.
- **Verifikasi OAuth Google** bila consent screen ingin keluar dari status *Testing*
  (scope Drive tergolong sensitif dan butuh peninjauan Google).
- Proses **peninjauan dan rilis** ke jalur internal/tertutup/terbuka/produksi.

---

## Pengujian

```bash
cd backend
go vet ./...
go test ./...      # unit + integrasi HTTP di atas datastore in-memory
```

Uji integrasi PostgreSQL berjalan hanya bila `POS_TEST_DATABASE_URL` disetel, sehingga
`go test ./...` tetap hijau di mesin tanpa database:

```bash
docker compose up -d
POS_TEST_DATABASE_URL="postgres://pos:pos@localhost:5432/pos_steca?sslmode=disable" \
  go test ./internal/repository/postgres/ -v
```

```bash
cd frontend
npm run typecheck  # TypeScript strict
npm run build
```

Cakupan pengujian backend meliputi agregasi laporan, alur kasir dan pesanan, akumulasi poin
loyalitas, siklus status meja, penyimpanan tenant terenkripsi, normalisasi nomor HP, serta
uji integrasi HTTP untuk RBAC, checkout, dan kanal pesanan online.

Khusus layer PostgreSQL:

- **Unit** (tanpa database): pemetaan baris hasil query, penyusunan filter pesanan, pemetaan
  kode error driver, pemuatan/pengurutan migrasi, dan **penjagaan multi-tenant** yang memeriksa
  setiap perintah SQL benar-benar disaring `tenant_id`.
- **Integrasi** (butuh PostgreSQL): migrasi idempoten, CRUD tiap entitas, enkripsi refresh
  token di kolom database, keunikan SKU/email/nomor HP, `ON DELETE CASCADE` saat tenant
  dihapus, dan **isolasi antar tenant** — dua tenant memakai ID entitas yang sama persis lalu
  dipastikan tidak bisa saling membaca maupun mengubah.

---

## Catatan desain dan batasan

- **QRIS dan kartu masih placeholder.** Transaksi dicatat lunas tanpa memanggil payment
  gateway. Integrasi nyata cukup ditambahkan di `service.OrderService.Checkout` sebelum
  transaksi ditulis.
- **Konkurensi.** Penyesuaian stok dihitung di database (`GREATEST(stock + $3, 0)` di dalam
  transaksi) sehingga aman untuk banyak kasir maupun banyak instance aplikasi sekaligus.
- **Kuota API Google.** Hanya unggah/hapus gambar produk yang menyentuh Google Drive, jadi
  lalu lintasnya kecil dan tidak ada cache katalog yang perlu dijaga. Pembacaan data
  sehari-hari sepenuhnya dilayani PostgreSQL.
- **Poin loyalitas** dihitung 1 poin per Rp 10.000 (`service.RupiahPerPoint`), dibulatkan
  ke bawah, dan hanya diberikan setelah pembayaran diterima — pesanan online baru
  mendapat poin ketika kasir menyelesaikan pembayarannya. Pelanggan dikenali dari nomor
  HP yang dinormalkan (`0812…`, `+62812…`, dan `62812…` dianggap sama).
- **Status meja** berpindah otomatis: terisi saat pesanan dibuat di meja tersebut, lalu
  perlu dibersihkan ketika pesanannya selesai atau dibatalkan. Staf menandainya kosong
  setelah dirapikan. Meja yang sudah dipakai pesanan lain tidak ikut terbebaskan.
- **Stok dipotong saat pesanan dibuat** (baik dari kasir maupun online) dan dikembalikan
  saat pesanan dibatalkan. Bila penulisan stok gagal setelah transaksi tercatat, struk tetap
  diterbitkan — penjualan yang sudah terjadi tidak boleh hilang karena kegagalan sinkronisasi.
- **Gambar produk diberi izin baca publik** di Drive agar bisa ditampilkan pada tag `<img>`.
  Tautannya sulit ditebak, tetapi siapa pun yang memegang tautan tersebut bisa membukanya.
- **Zona waktu** default WIB (UTC+7) dan bisa diubah lewat `APP_TIMEZONE`. Waktu disimpan
  sebagai `TIMESTAMPTZ` (UTC) di database dan ditampilkan mengikuti zona waktu aplikasi,
  sehingga laporan harian tidak bergeser.
