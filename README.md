# Steca POS

Aplikasi **Point of Sale untuk UMKM Indonesia** — terinspirasi kombinasi Majoo, Moka POS,
dan modul manajemen pesanan F&B ala Trofi.

Yang membedakan: **tidak ada database SQL/NoSQL sama sekali**. Setiap akun bisnis memakai
**Google Drive dan Google Sheets miliknya sendiri** sebagai datastore. Folder dan
spreadsheet dibuat otomatis saat onboarding, dan datanya tetap sepenuhnya milik pemilik usaha.

```
┌──────────────┐   REST/JSON    ┌──────────────┐   OAuth2 per tenant   ┌────────────────┐
│  React SPA   │ ─────────────► │  Backend Go  │ ────────────────────► │ Google Drive   │
│ Vite + TS    │ ◄───────────── │  Gin, modular│ ◄──────────────────── │ Google Sheets  │
└──────────────┘                └──────────────┘                       └────────────────┘
```

---

## Daftar isi

- [Fitur](#fitur)
- [Tech stack](#tech-stack)
- [Struktur data di Google Drive](#struktur-data-di-google-drive)
- [Struktur project](#struktur-project)
- [Menjalankan cepat (mode demo, tanpa akun Google)](#menjalankan-cepat-mode-demo-tanpa-akun-google)
- [Setup Google Cloud (mode produksi)](#setup-google-cloud-mode-produksi)
- [Menjalankan mode produksi](#menjalankan-mode-produksi)
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
| **Inventori/produk** | CRUD produk yang sinkron dengan sheet `Products`, termasuk unggah foto produk ke folder Drive tenant. |
| **Laporan penjualan** | Dibaca dari sheet `Transactions`: harian/mingguan/bulanan, produk terlaris, total omzet, metode pembayaran, performa kasir. |
| **CRM & loyalitas** | Sheet `Customers` per tenant. Kasir boleh menautkan pelanggan saat checkout — opsional, transaksi anonim tetap sah. Poin terkumpul otomatis 1 poin per Rp 10.000 belanja, dan owner punya halaman pelanggan lengkap dengan riwayat pembelian. |
| **Denah / nomor meja** | Sheet `Tables` berisi nomor/nama meja dan kapasitasnya. Status `kosong / terisi / perlu dibersihkan` terlihat di layar kasir dan papan pesanan, dan setiap transaksi bisa dikaitkan ke meja tertentu. |
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
- **Datastore** — Google Drive API v3 + Google Sheets API v4, OAuth2 per akun pengguna.
- **Android** — [Capacitor](https://capacitorjs.com/) membungkus build web yang sama menjadi
  APK/AAB (`id.stecapos.app`), tanpa menulis ulang UI. Lihat
  [Aplikasi Android](#aplikasi-android-capacitor).

---

## Struktur data di Google Drive

Saat pemilik pertama kali masuk dengan Google, backend otomatis membuat:

```
Drive pemilik akun/
└── POS Steca - <Nama Bisnis>/          ← folder khusus tenant
    ├── POS Steca Data - <Nama Bisnis>  ← Google Spreadsheet (datastore)
    ├── produk-20260809-150405-a1b2c.jpg
    └── produk-...                       ← seluruh foto produk tenant
```

Spreadsheet berisi enam sheet:

**`Products`**

| ID | Nama | Kategori | Harga | Stok | SKU | URL Gambar | ID Gambar |
|---|---|---|---|---|---|---|---|

> Kolom *ID Gambar* menyimpan file ID Drive agar gambar lama bisa dibersihkan
> saat produk diubah atau dihapus.

**`Transactions`** — append-only, satu baris per item terjual

| Tanggal | ID Transaksi | Item | Qty | Harga Satuan | Total | Metode Pembayaran | Kasir |
|---|---|---|---|---|---|---|---|

**`Orders`** — modul manajemen pesanan

| ID Pesanan | Kode | ID Transaksi | Tanggal | Diperbarui | Status | Sumber | Nama Pelanggan | No Meja | Items (JSON) | Total | Catatan | Kasir | ID Pelanggan | ID Meja |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|

**`Employees`**

| ID | Nama | Email | Role | Status | PIN Hash | Dibuat |
|---|---|---|---|---|---|---|

**`Customers`** — CRM & loyalitas

| ID | Nama | No HP | Total Belanja | Poin | Terakhir Belanja | Dibuat |
|---|---|---|---|---|---|---|

**`Tables`** — denah meja F&B

| ID | Nama Meja | Kapasitas | Status | Area | ID Pesanan Aktif | Diperbarui |
|---|---|---|---|---|---|---|

> Kolom baru selalu ditambahkan di ujung kanan, dan aplikasi otomatis
> melengkapi baris header lama saat pemilik login kembali — spreadsheet tenant
> yang sudah berisi data tidak perlu dimigrasi manual.

Spreadsheet ini tetap bisa dibuka, difilter, dan diekspor langsung oleh pemilik usaha dari
Google Sheets — perubahan manual yang wajar (misal mengetik `Rp 15.000` pada kolom harga)
tetap terbaca oleh aplikasi.

### Satu-satunya data di luar Google

Backend perlu tahu spreadsheet mana milik siapa **sebelum** bisa memanggil Google API. Karena
itu ada satu berkas `data/tenants.json` yang menyimpan daftar tenant beserta refresh token
Google-nya. Refresh token **selalu terenkripsi AES-256-GCM** memakai `ENCRYPTION_KEY`, tidak
pernah tersimpan sebagai teks biasa, dan tidak pernah dikirim ke frontend.

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
    ├── googleapi/                  # pembungkus OAuth2, Drive, Sheets + retry
    ├── repository/
    │   ├── gsheets/                # implementasi port di atas Sheets & Drive
    │   ├── memory/                 # driver in-memory (khusus pengembangan)
    │   └── tenantstore/            # daftar tenant di disk, token terenkripsi
    ├── service/                    # aturan bisnis: auth, kasir, pesanan, laporan…
    └── handler/                    # routing Gin, middleware, RBAC, validasi

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
2. **APIs & Services → Library** → aktifkan **Google Drive API** dan **Google Sheets API**.
3. **APIs & Services → OAuth consent screen**:
   - User type: *External* (atau *Internal* bila memakai Google Workspace).
   - Tambahkan scope: `.../auth/drive.file`, `.../auth/spreadsheets`, `openid`,
     `.../auth/userinfo.email`, `.../auth/userinfo.profile`.
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
# isi GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, JWT_SECRET, ENCRYPTION_KEY
# (JWT_SECRET dan ENCRYPTION_KEY: openssl rand -base64 48)

set -a && source .env && set +a
go run ./cmd/server
```

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

Backend akan menolak start bila `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `JWT_SECRET`,
atau `ENCRYPTION_KEY` belum diisi pada mode `google` — supaya salah konfigurasi ketahuan
saat deploy, bukan saat pengguna pertama login.

Seluruh variabel env terdokumentasi di [`backend/.env.example`](backend/.env.example).

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
  mengakses Google Drive dan Sheets milik pengguna.
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

```bash
cd frontend
npm run typecheck  # TypeScript strict
npm run build
```

Cakupan pengujian backend meliputi pemetaan baris Sheets ↔ model (termasuk toleransi format
rupiah), agregasi laporan, alur kasir dan pesanan, akumulasi poin loyalitas, siklus status
meja, penyimpanan tenant terenkripsi, serta uji integrasi HTTP untuk RBAC, checkout, dan
kanal pesanan online.

---

## Catatan desain dan batasan

- **QRIS dan kartu masih placeholder.** Transaksi dicatat lunas tanpa memanggil payment
  gateway. Integrasi nyata cukup ditambahkan di `service.OrderService.Checkout` sebelum
  transaksi ditulis.
- **Google Sheets tidak punya transaksi ACID.** Operasi baca-ubah-tulis diserialkan per
  tenant memakai mutex di dalam proses (`gsheets.Provider.Lock`). Untuk deployment banyak
  instance, kunci ini perlu dipindah ke penyimpanan bersama (misal Redis).
- **Kuota API.** Katalog produk dibaca sangat sering oleh layar kasir, jadi hasilnya
  di-cache singkat (`SHEETS_CACHE_TTL`, default 20 detik) dan seluruh panggilan Google
  memakai retry eksponensial untuk status 429/5xx.
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
- **Zona waktu** default WIB (UTC+7) dan bisa diubah lewat `APP_TIMEZONE`. Tanggal ditulis
  ke sheet dalam format RFC3339 lengkap dengan offset agar tidak ambigu saat diekspor.
