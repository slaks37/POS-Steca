-- Skema awal Steca POS di PostgreSQL.
--
-- Seluruh tabel entitas memakai kunci utama gabungan (tenant_id, id) sehingga
-- pemisahan data antar bisnis terjadi di level skema, bukan sekadar konvensi
-- query. tenant_id selalu mengacu ke tabel tenants dengan ON DELETE CASCADE,
-- jadi menghapus satu tenant membersihkan seluruh datanya.

-- ---------------------------------------------------------------------------
-- tenants: identitas tiap akun bisnis.
-- refresh_token_enc berisi ciphertext AES-256-GCM (base64 URL-safe) yang
-- dihasilkan internal/crypto — database tidak pernah menyimpan token mentah.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tenants (
    id                TEXT        PRIMARY KEY,
    code              TEXT        NOT NULL,
    business_name     TEXT        NOT NULL,
    owner_email       TEXT        NOT NULL,
    owner_name        TEXT        NOT NULL DEFAULT '',
    folder_id         TEXT        NOT NULL DEFAULT '',
    spreadsheet_id    TEXT        NOT NULL DEFAULT '',
    refresh_token_enc TEXT        NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Kode bisnis dan email pemilik dibandingkan tanpa memperhatikan huruf besar/kecil.
CREATE UNIQUE INDEX IF NOT EXISTS tenants_code_key ON tenants (upper(code));
CREATE UNIQUE INDEX IF NOT EXISTS tenants_owner_email_key ON tenants (lower(owner_email));

-- ---------------------------------------------------------------------------
-- employees
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS employees (
    tenant_id  TEXT        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id         TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    email      TEXT        NOT NULL,
    role       TEXT        NOT NULL CHECK (role IN ('owner', 'kasir')),
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    pin_hash   TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);

CREATE UNIQUE INDEX IF NOT EXISTS employees_tenant_email_key
    ON employees (tenant_id, lower(email));

-- ---------------------------------------------------------------------------
-- products
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS products (
    tenant_id  TEXT          NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id         TEXT          NOT NULL,
    name       TEXT          NOT NULL,
    category   TEXT          NOT NULL DEFAULT '',
    price      NUMERIC(14, 2) NOT NULL DEFAULT 0 CHECK (price >= 0),
    stock      INTEGER       NOT NULL DEFAULT 0 CHECK (stock >= 0),
    sku        TEXT          NOT NULL DEFAULT '',
    image_url  TEXT          NOT NULL DEFAULT '',
    image_id   TEXT          NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ   NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);

-- SKU wajib unik per tenant, tetapi boleh dikosongkan.
CREATE UNIQUE INDEX IF NOT EXISTS products_tenant_sku_key
    ON products (tenant_id, lower(sku)) WHERE sku <> '';

CREATE INDEX IF NOT EXISTS products_tenant_name_idx ON products (tenant_id, lower(name));

-- ---------------------------------------------------------------------------
-- customers (CRM & loyalitas)
-- phone_normalized diisi aplikasi memakai domain.NormalizePhone agar aturan
-- "0812…", "+62812…", dan "62812…" dianggap satu pelanggan tetap satu sumber.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS customers (
    tenant_id        TEXT           NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id               TEXT           NOT NULL,
    name             TEXT           NOT NULL,
    phone            TEXT           NOT NULL DEFAULT '',
    phone_normalized TEXT           NOT NULL DEFAULT '',
    total_spent      NUMERIC(14, 2) NOT NULL DEFAULT 0,
    points           INTEGER        NOT NULL DEFAULT 0,
    last_purchase    TIMESTAMPTZ,
    created_at       TIMESTAMPTZ    NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);

CREATE UNIQUE INDEX IF NOT EXISTS customers_tenant_phone_key
    ON customers (tenant_id, phone_normalized) WHERE phone_normalized <> '';

CREATE INDEX IF NOT EXISTS customers_tenant_spent_idx
    ON customers (tenant_id, total_spent DESC);

-- ---------------------------------------------------------------------------
-- tables (denah meja F&B)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS tables (
    tenant_id       TEXT        NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id              TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    capacity        INTEGER     NOT NULL DEFAULT 0 CHECK (capacity >= 0),
    status          TEXT        NOT NULL DEFAULT 'kosong'
                                CHECK (status IN ('kosong', 'terisi', 'dibersihkan')),
    area            TEXT        NOT NULL DEFAULT '',
    active_order_id TEXT        NOT NULL DEFAULT '',
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);

CREATE UNIQUE INDEX IF NOT EXISTS tables_tenant_name_key
    ON tables (tenant_id, lower(name));

-- ---------------------------------------------------------------------------
-- orders (modul manajemen pesanan)
-- Daftar item disimpan sebagai JSONB agar satu pesanan tetap satu baris.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS orders (
    tenant_id      TEXT           NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    id             TEXT           NOT NULL,
    code           TEXT           NOT NULL DEFAULT '',
    transaction_id TEXT           NOT NULL DEFAULT '',
    status         TEXT           NOT NULL
                                  CHECK (status IN ('baru', 'diproses', 'selesai', 'batal')),
    source         TEXT           NOT NULL CHECK (source IN ('kasir', 'online')),
    customer_id    TEXT           NOT NULL DEFAULT '',
    customer_name  TEXT           NOT NULL DEFAULT '',
    table_id       TEXT           NOT NULL DEFAULT '',
    table_no       TEXT           NOT NULL DEFAULT '',
    items          JSONB          NOT NULL DEFAULT '[]'::jsonb,
    total          NUMERIC(14, 2) NOT NULL DEFAULT 0,
    note           TEXT           NOT NULL DEFAULT '',
    cashier        TEXT           NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ    NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX IF NOT EXISTS orders_tenant_created_idx ON orders (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS orders_tenant_status_idx ON orders (tenant_id, status);
CREATE INDEX IF NOT EXISTS orders_tenant_source_idx ON orders (tenant_id, source);
CREATE INDEX IF NOT EXISTS orders_tenant_customer_idx
    ON orders (tenant_id, customer_id) WHERE customer_id <> '';

-- ---------------------------------------------------------------------------
-- transaction_lines: bersifat append-only, satu baris per item terjual.
-- Laporan penjualan membacanya.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS transaction_lines (
    seq            BIGSERIAL      PRIMARY KEY,
    tenant_id      TEXT           NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    occurred_at    TIMESTAMPTZ    NOT NULL,
    transaction_id TEXT           NOT NULL,
    item           TEXT           NOT NULL,
    qty            INTEGER        NOT NULL,
    unit_price     NUMERIC(14, 2) NOT NULL DEFAULT 0,
    total          NUMERIC(14, 2) NOT NULL DEFAULT 0,
    payment_method TEXT           NOT NULL DEFAULT '',
    cashier        TEXT           NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS transaction_lines_tenant_time_idx
    ON transaction_lines (tenant_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS transaction_lines_tenant_trx_idx
    ON transaction_lines (tenant_id, transaction_id);
