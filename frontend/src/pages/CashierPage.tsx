import { useCallback, useEffect, useMemo, useState } from 'react'

import { CustomerPicker } from '../components/CustomerPicker'
import { ReceiptView } from '../components/ReceiptView'
import { EmptyState, ErrorAlert, LoadingCards, Modal, Toast, tableStatusLabel } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { ApiError, request } from '../lib/api'
import type { Envelope } from '../lib/api'
import { formatRupiah } from '../lib/format'
import type { CheckoutResult, Customer, PaymentMethod, Product, Table } from '../lib/types'

interface CartLine {
  product: Product
  qty: number
}

const paymentOptions: { value: PaymentMethod; label: string }[] = [
  { value: 'tunai', label: 'Tunai' },
  { value: 'qris', label: 'QRIS' },
  { value: 'kartu', label: 'Kartu' },
]

const quickCash = [10000, 20000, 50000, 100000]

/** CashierPage adalah layar transaksi utama kasir, dioptimalkan untuk tablet. */
export function CashierPage() {
  const { tenant } = useAuth()
  const [products, setProducts] = useState<Product[]>([])
  const [categories, setCategories] = useState<string[]>([])
  const [tables, setTables] = useState<Table[]>([])
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [toast, setToast] = useState<string | null>(null)

  const [cart, setCart] = useState<CartLine[]>([])
  const [payment, setPayment] = useState<PaymentMethod>('tunai')
  const [amountPaid, setAmountPaid] = useState('')
  const [customer, setCustomer] = useState<Customer | null>(null)
  const [customerName, setCustomerName] = useState('')
  const [customerPhone, setCustomerPhone] = useState('')
  const [tableId, setTableId] = useState('')
  const [note, setNote] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [result, setResult] = useState<CheckoutResult | null>(null)

  const load = useCallback(async () => {
    setError(null)
    try {
      const [items, cats, tbl] = await Promise.all([
        request<Envelope<Product[]>>('/products'),
        request<Envelope<string[]>>('/products/categories'),
        request<Envelope<Table[]>>('/tables'),
      ])
      setProducts(items.data ?? [])
      setCategories(cats.data ?? [])
      setTables(tbl.data ?? [])
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal memuat katalog produk')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    if (!toast) return
    const timer = window.setTimeout(() => setToast(null), 2600)
    return () => window.clearTimeout(timer)
  }, [toast])

  const visibleProducts = useMemo(() => {
    const q = query.trim().toLowerCase()
    return products.filter((p) => {
      if (category && p.category !== category) return false
      if (!q) return true
      return `${p.name} ${p.sku} ${p.category}`.toLowerCase().includes(q)
    })
  }, [products, query, category])

  const total = useMemo(() => cart.reduce((sum, line) => sum + line.product.price * line.qty, 0), [cart])
  const itemCount = cart.reduce((sum, line) => sum + line.qty, 0)
  const paid = Number(amountPaid.replace(/[^\d]/g, '')) || 0
  const change = payment === 'tunai' && paid > total ? paid - total : 0
  const insufficient = payment === 'tunai' && paid > 0 && paid < total
  const estimatedPoints = Math.floor(total / 10000)
  const willEarnPoints = Boolean(customer || customerPhone.trim())

  function addToCart(product: Product) {
    setCart((current) => {
      const existing = current.find((line) => line.product.id === product.id)
      if (existing) {
        if (existing.qty >= product.stock) return current
        return current.map((line) => (line.product.id === product.id ? { ...line, qty: line.qty + 1 } : line))
      }
      return [...current, { product, qty: 1 }]
    })
  }

  function changeQty(productId: string, delta: number) {
    setCart((current) =>
      current
        .map((line) => {
          if (line.product.id !== productId) return line
          return { ...line, qty: Math.min(Math.max(line.qty + delta, 0), line.product.stock) }
        })
        .filter((line) => line.qty > 0),
    )
  }

  function resetCart() {
    setCart([])
    setAmountPaid('')
    setCustomer(null)
    setCustomerName('')
    setCustomerPhone('')
    setTableId('')
    setNote('')
    setPayment('tunai')
  }

  async function handleCheckout() {
    if (cart.length === 0) return
    setSubmitting(true)
    setError(null)
    try {
      const res = await request<Envelope<CheckoutResult>>('/checkout', {
        method: 'POST',
        body: {
          items: cart.map((line) => ({ product_id: line.product.id, qty: line.qty })),
          payment_method: payment,
          amount_paid: payment === 'tunai' ? paid : 0,
          customer_id: customer?.id ?? '',
          customer_name: customer?.name ?? customerName,
          customer_phone: customer ? '' : customerPhone,
          table_id: tableId,
          note,
        },
      })
      setResult(res.data)
      resetCart()
      void load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Transaksi gagal disimpan')
    } finally {
      setSubmitting(false)
    }
  }

  async function markTableClean(table: Table) {
    try {
      await request<Envelope<Table>>(`/tables/${table.id}/status`, {
        method: 'PATCH',
        body: { status: 'kosong' },
      })
      setToast(`${table.name} siap dipakai lagi`)
      void load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal memperbarui status meja')
    }
  }

  const dirtyTables = tables.filter((t) => t.status === 'dibersihkan')

  return (
    <div className="pos-layout">
      <div className="stack">
        <div className="card card-pad">
          <div className="row">
            <input
              style={{ flex: '2 1 220px' }}
              placeholder="Cari produk atau SKU..."
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              aria-label="Cari produk"
            />
            <select
              style={{ flex: '1 1 160px' }}
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              aria-label="Filter kategori"
            >
              <option value="">Semua kategori</option>
              {categories.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
            <button type="button" className="btn btn-secondary" onClick={() => void load()}>
              Muat ulang
            </button>
          </div>
        </div>

        <ErrorAlert message={error} />

        {dirtyTables.length > 0 ? (
          <div className="card card-pad">
            <div className="row">
              <strong className="small">🧹 Meja perlu dibersihkan</strong>
              {dirtyTables.map((table) => (
                <button
                  key={table.id}
                  type="button"
                  className="btn btn-secondary btn-sm"
                  onClick={() => void markTableClean(table)}
                >
                  {table.name} • tandai kosong
                </button>
              ))}
            </div>
          </div>
        ) : null}

        {loading ? (
          <LoadingCards count={6} tile />
        ) : visibleProducts.length === 0 ? (
          <div className="card">
            <EmptyState
              icon={products.length === 0 ? '📦' : '🔍'}
              title={products.length === 0 ? 'Belum ada produk' : 'Produk tidak ditemukan'}
              hint={
                products.length === 0
                  ? 'Tambahkan produk lewat menu Produk agar bisa mulai berjualan.'
                  : 'Coba kata kunci lain atau ganti filter kategori.'
              }
            />
          </div>
        ) : (
          <div className="product-grid">
            {visibleProducts.map((product) => {
              const inCart = cart.find((line) => line.product.id === product.id)?.qty ?? 0
              const habis = product.stock <= 0
              return (
                <button
                  type="button"
                  key={product.id}
                  className="product-card"
                  onClick={() => addToCart(product)}
                  disabled={habis || inCart >= product.stock}
                >
                  <div className="product-thumb-wrap">
                    {product.image_url ? (
                      <img className="product-thumb" src={product.image_url} alt={product.name} loading="lazy" />
                    ) : (
                      <div className="product-thumb-placeholder" aria-hidden>
                        {product.name.slice(0, 2).toUpperCase()}
                      </div>
                    )}
                    {inCart > 0 ? <span className="product-qty-flag">{inCart}</span> : null}
                  </div>
                  <div className="product-body">
                    <span className="product-name">{product.name}</span>
                    <span className="product-price">{formatRupiah(product.price)}</span>
                    <span className="tiny muted">{habis ? 'Stok habis' : `Stok ${product.stock}`}</span>
                  </div>
                </button>
              )
            })}
          </div>
        )}
      </div>

      <div className="card cart">
        <div className="card-header">
          <h3>Keranjang {itemCount > 0 ? <span className="badge">{itemCount}</span> : null}</h3>
          {cart.length > 0 ? (
            <button type="button" className="btn btn-ghost btn-sm" onClick={resetCart}>
              Kosongkan
            </button>
          ) : null}
        </div>

        <div className="cart-items">
          {cart.length === 0 ? (
            <EmptyState icon="🛒" title="Keranjang kosong" hint="Ketuk produk di sebelah kiri untuk menambahkan." />
          ) : (
            cart.map((line) => (
              <div className="cart-item" key={line.product.id}>
                <div>
                  <div style={{ fontWeight: 600 }}>{line.product.name}</div>
                  <div className="tiny muted">{formatRupiah(line.product.price)} / item</div>
                </div>
                <div style={{ textAlign: 'right', fontWeight: 650 }} className="num">
                  {formatRupiah(line.product.price * line.qty)}
                </div>
                <div className="qty-control">
                  <button type="button" onClick={() => changeQty(line.product.id, -1)} aria-label="Kurangi">
                    −
                  </button>
                  <span>{line.qty}</span>
                  <button
                    type="button"
                    onClick={() => changeQty(line.product.id, 1)}
                    aria-label="Tambah"
                    disabled={line.qty >= line.product.stock}
                  >
                    +
                  </button>
                </div>
              </div>
            ))
          )}
        </div>

        <div className="cart-summary">
          <div className="field">
            <label>Pelanggan</label>
            <CustomerPicker
              selected={customer}
              onSelect={setCustomer}
              name={customerName}
              phone={customerPhone}
              onNameChange={setCustomerName}
              onPhoneChange={setCustomerPhone}
            />
          </div>

          {tables.length > 0 ? (
            <div className="field">
              <label>Meja</label>
              <div className="table-chips">
                <button
                  type="button"
                  className={`table-chip${tableId === '' ? ' active' : ''}`}
                  onClick={() => setTableId('')}
                >
                  Tanpa meja
                  <span className="cap">bawa pulang</span>
                </button>
                {tables.map((table) => (
                  <button
                    key={table.id}
                    type="button"
                    className={`table-chip is-${table.status}${tableId === table.id ? ' active' : ''}`}
                    onClick={() => setTableId(table.id === tableId ? '' : table.id)}
                    title={tableStatusLabel(table.status)}
                  >
                    {table.name}
                    <span className="cap">
                      {table.capacity > 0 ? `${table.capacity} kursi • ` : ''}
                      {tableStatusLabel(table.status)}
                    </span>
                  </button>
                ))}
              </div>
            </div>
          ) : null}

          <div className="field">
            <label>Metode pembayaran</label>
            <div className="choice-grid">
              {paymentOptions.map((option) => (
                <button
                  type="button"
                  key={option.value}
                  className={`choice${payment === option.value ? ' active' : ''}`}
                  onClick={() => setPayment(option.value)}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          {payment === 'tunai' ? (
            <div className="field">
              <label htmlFor="paid">Uang diterima</label>
              <input
                id="paid"
                inputMode="numeric"
                value={amountPaid}
                onChange={(e) => setAmountPaid(e.target.value)}
                placeholder="0"
              />
              <div className="row row-tight" style={{ marginTop: 8 }}>
                {quickCash.map((amount) => (
                  <button
                    type="button"
                    key={amount}
                    className="btn btn-secondary btn-sm"
                    onClick={() => setAmountPaid(String(amount))}
                  >
                    {formatRupiah(amount)}
                  </button>
                ))}
                <button type="button" className="btn btn-secondary btn-sm" onClick={() => setAmountPaid(String(total))}>
                  Uang pas
                </button>
              </div>
            </div>
          ) : (
            <div className="alert alert-info small">
              <span aria-hidden>ℹ️</span>
              <span>
                {payment === 'qris' ? 'QRIS' : 'Kartu'} masih placeholder: transaksi dicatat lunas tanpa memanggil
                payment gateway.
              </span>
            </div>
          )}

          <div className="field">
            <label htmlFor="note">Catatan pesanan</label>
            <input id="note" value={note} onChange={(e) => setNote(e.target.value)} placeholder="Misal: tanpa sambal" />
          </div>

          <div className="summary-row">
            <span className="muted">Jumlah item</span>
            <span className="num">{itemCount}</span>
          </div>
          <div className="summary-row">
            <span className="muted">Total</span>
            <span className="summary-total">{formatRupiah(total)}</span>
          </div>
          {payment === 'tunai' && paid > 0 ? (
            <div className="summary-row">
              <span className="muted">Kembalian</span>
              <span
                className="num"
                style={{ fontWeight: 700, color: insufficient ? 'var(--danger)' : 'var(--success)' }}
              >
                {insufficient ? 'Uang kurang' : formatRupiah(change)}
              </span>
            </div>
          ) : null}
          {willEarnPoints && estimatedPoints > 0 ? (
            <div className="summary-row">
              <span className="muted">Poin loyalitas</span>
              <span className="badge badge-loyal">+{estimatedPoints} poin</span>
            </div>
          ) : null}

          <button
            type="button"
            className="btn btn-lg btn-block"
            style={{ marginTop: 12 }}
            disabled={cart.length === 0 || submitting || insufficient}
            onClick={() => void handleCheckout()}
          >
            {submitting ? (
              <>
                <span className="spinner spinner-light" />
                Menyimpan...
              </>
            ) : (
              `Bayar ${formatRupiah(total)}`
            )}
          </button>
        </div>
      </div>

      {result?.receipt ? (
        <Modal
          title="Transaksi berhasil"
          onClose={() => setResult(null)}
          footer={
            <>
              <button type="button" className="btn btn-secondary" onClick={() => window.print()}>
                Cetak struk
              </button>
              <button type="button" className="btn" onClick={() => setResult(null)}>
                Selesai
              </button>
            </>
          }
        >
          <ReceiptView
            receipt={result.receipt}
            order={result.order}
            businessName={tenant?.business_name ?? 'Steca POS'}
          />
        </Modal>
      ) : null}

      {toast ? <Toast message={toast} /> : null}
    </div>
  )
}
