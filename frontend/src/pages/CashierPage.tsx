import { useCallback, useEffect, useMemo, useState } from 'react'

import { ReceiptView } from '../components/ReceiptView'
import { EmptyState, ErrorAlert, LoadingRows, Modal } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { ApiError, request } from '../lib/api'
import type { Envelope } from '../lib/api'
import { formatRupiah } from '../lib/format'
import type { CheckoutResult, PaymentMethod, Product } from '../lib/types'

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

/** CashierPage adalah layar transaksi utama kasir. */
export function CashierPage() {
  const { tenant } = useAuth()
  const [products, setProducts] = useState<Product[]>([])
  const [categories, setCategories] = useState<string[]>([])
  const [query, setQuery] = useState('')
  const [category, setCategory] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [cart, setCart] = useState<CartLine[]>([])
  const [payment, setPayment] = useState<PaymentMethod>('tunai')
  const [amountPaid, setAmountPaid] = useState('')
  const [customerName, setCustomerName] = useState('')
  const [tableNo, setTableNo] = useState('')
  const [note, setNote] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [result, setResult] = useState<CheckoutResult | null>(null)

  const loadProducts = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [items, cats] = await Promise.all([
        request<Envelope<Product[]>>('/products'),
        request<Envelope<string[]>>('/products/categories'),
      ])
      setProducts(items.data ?? [])
      setCategories(cats.data ?? [])
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal memuat katalog produk')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadProducts()
  }, [loadProducts])

  const visibleProducts = useMemo(() => {
    const q = query.trim().toLowerCase()
    return products.filter((p) => {
      if (category && p.category !== category) return false
      if (!q) return true
      return `${p.name} ${p.sku} ${p.category}`.toLowerCase().includes(q)
    })
  }, [products, query, category])

  const total = useMemo(() => cart.reduce((sum, line) => sum + line.product.price * line.qty, 0), [cart])
  const paid = Number(amountPaid.replace(/[^\d]/g, '')) || 0
  const change = payment === 'tunai' && paid > total ? paid - total : 0
  const insufficient = payment === 'tunai' && paid > 0 && paid < total

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
          const nextQty = Math.min(Math.max(line.qty + delta, 0), line.product.stock)
          return { ...line, qty: nextQty }
        })
        .filter((line) => line.qty > 0),
    )
  }

  function resetCart() {
    setCart([])
    setAmountPaid('')
    setCustomerName('')
    setTableNo('')
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
          customer_name: customerName,
          table_no: tableNo,
          note,
        },
      })
      setResult(res.data)
      resetCart()
      void loadProducts()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Transaksi gagal disimpan')
    } finally {
      setSubmitting(false)
    }
  }

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
            <button type="button" className="btn btn-secondary" onClick={() => void loadProducts()}>
              Muat ulang
            </button>
          </div>
        </div>

        <ErrorAlert message={error} />

        {loading ? (
          <div className="card">
            <LoadingRows rows={5} />
          </div>
        ) : visibleProducts.length === 0 ? (
          <div className="card">
            <EmptyState title="Belum ada produk" hint="Tambahkan produk lewat menu Produk & Stok." />
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
                  {product.image_url ? (
                    <img className="product-thumb" src={product.image_url} alt={product.name} loading="lazy" />
                  ) : (
                    <div className="product-thumb-placeholder" aria-hidden>
                      {product.name.slice(0, 2).toUpperCase()}
                    </div>
                  )}
                  <div className="product-body">
                    <span className="product-name">{product.name}</span>
                    <span className="product-price">{formatRupiah(product.price)}</span>
                    <span className="tiny muted">
                      {habis ? 'Stok habis' : `Stok ${product.stock}`}
                      {inCart > 0 ? ` • ${inCart} di keranjang` : ''}
                    </span>
                  </div>
                </button>
              )
            })}
          </div>
        )}
      </div>

      <div className="card cart">
        <div className="card-header">
          <h3>Keranjang</h3>
          {cart.length > 0 ? (
            <button type="button" className="btn btn-ghost btn-sm" onClick={resetCart}>
              Kosongkan
            </button>
          ) : null}
        </div>

        <div className="cart-items">
          {cart.length === 0 ? (
            <EmptyState title="Keranjang kosong" hint="Pilih produk di sebelah kiri untuk mulai transaksi." />
          ) : (
            cart.map((line) => (
              <div className="cart-item" key={line.product.id}>
                <div>
                  <div style={{ fontWeight: 600 }}>{line.product.name}</div>
                  <div className="tiny muted">{formatRupiah(line.product.price)} / item</div>
                </div>
                <div style={{ textAlign: 'right', fontWeight: 650 }}>
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
          <div className="field-row">
            <div className="field" style={{ marginBottom: 8 }}>
              <label htmlFor="customer">Nama pelanggan</label>
              <input id="customer" value={customerName} onChange={(e) => setCustomerName(e.target.value)} placeholder="Opsional" />
            </div>
            <div className="field" style={{ marginBottom: 8 }}>
              <label htmlFor="table">No. meja</label>
              <input id="table" value={tableNo} onChange={(e) => setTableNo(e.target.value)} placeholder="Opsional" />
            </div>
          </div>

          <label>Metode pembayaran</label>
          <div className="pay-methods">
            {paymentOptions.map((option) => (
              <button
                type="button"
                key={option.value}
                className={`pay-method${payment === option.value ? ' active' : ''}`}
                onClick={() => setPayment(option.value)}
              >
                {option.label}
              </button>
            ))}
          </div>

          {payment === 'tunai' ? (
            <div className="field" style={{ marginBottom: 8 }}>
              <label htmlFor="paid">Uang diterima</label>
              <input
                id="paid"
                inputMode="numeric"
                value={amountPaid}
                onChange={(e) => setAmountPaid(e.target.value)}
                placeholder="0"
              />
              <div className="row" style={{ gap: 6, marginTop: 6 }}>
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
            <div className="alert alert-info small" style={{ marginBottom: 8 }}>
              {payment === 'qris' ? 'QRIS' : 'Kartu'} masih placeholder: transaksi dicatat lunas tanpa memanggil
              payment gateway.
            </div>
          )}

          <div className="field" style={{ marginBottom: 10 }}>
            <label htmlFor="note">Catatan pesanan</label>
            <input id="note" value={note} onChange={(e) => setNote(e.target.value)} placeholder="Misal: tanpa sambal" />
          </div>

          <div className="summary-row">
            <span className="muted">Jumlah item</span>
            <span>{cart.reduce((sum, line) => sum + line.qty, 0)}</span>
          </div>
          <div className="summary-row">
            <span className="muted">Total</span>
            <span className="summary-total">{formatRupiah(total)}</span>
          </div>
          {payment === 'tunai' && paid > 0 ? (
            <div className="summary-row">
              <span className="muted">Kembalian</span>
              <span style={{ fontWeight: 700, color: insufficient ? 'var(--danger)' : 'var(--success)' }}>
                {insufficient ? 'Uang kurang' : formatRupiah(change)}
              </span>
            </div>
          ) : null}

          <button
            type="button"
            className="btn btn-block"
            style={{ marginTop: 10 }}
            disabled={cart.length === 0 || submitting || insufficient}
            onClick={() => void handleCheckout()}
          >
            {submitting ? 'Menyimpan...' : `Bayar ${formatRupiah(total)}`}
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
            businessName={tenant?.business_name ?? 'POS Steca'}
          />
        </Modal>
      ) : null}
    </div>
  )
}
