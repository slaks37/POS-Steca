import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'

import { EmptyState, ErrorAlert, LoadingRows } from '../components/ui'
import { ApiError, request } from '../lib/api'
import { formatRupiah } from '../lib/format'
import type { MenuItem } from '../lib/types'

interface MenuResponse {
  data: MenuItem[]
  business: { name: string; code: string }
}

interface CreatedOrder {
  id: string
  code: string
  status: string
  total: number
}

/** PublicMenuPage adalah kanal pemesanan online tanpa login untuk pelanggan. */
export function PublicMenuPage() {
  const { code = '' } = useParams()
  const [menu, setMenu] = useState<MenuItem[]>([])
  const [business, setBusiness] = useState<string>('')
  const [cart, setCart] = useState<Record<string, number>>({})
  const [customerName, setCustomerName] = useState('')
  const [tableNo, setTableNo] = useState('')
  const [note, setNote] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [created, setCreated] = useState<CreatedOrder | null>(null)

  useEffect(() => {
    let active = true
    request<MenuResponse>(`/public/${encodeURIComponent(code)}/menu`, { auth: false })
      .then((res) => {
        if (!active) return
        setMenu(res.data ?? [])
        setBusiness(res.business?.name ?? '')
      })
      .catch((err) => {
        if (active) setError(err instanceof ApiError ? err.message : 'Menu tidak dapat dimuat')
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [code])

  const items = useMemo(() => menu.filter((item) => (cart[item.id] ?? 0) > 0), [menu, cart])
  const total = useMemo(
    () => items.reduce((sum, item) => sum + item.price * (cart[item.id] ?? 0), 0),
    [items, cart],
  )

  function changeQty(id: string, delta: number) {
    setCart((current) => {
      const next = Math.max((current[id] ?? 0) + delta, 0)
      const updated = { ...current, [id]: next }
      if (next === 0) delete updated[id]
      return updated
    })
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setSubmitting(true)
    setError(null)
    try {
      const res = await request<{ data: CreatedOrder }>(`/public/${encodeURIComponent(code)}/orders`, {
        method: 'POST',
        auth: false,
        body: {
          items: items.map((item) => ({ product_id: item.id, qty: cart[item.id] })),
          customer_name: customerName,
          table_no: tableNo,
          note,
        },
      })
      setCreated(res.data)
      setCart({})
      setNote('')
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Pesanan gagal dikirim')
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) {
    return (
      <div className="public-page">
        <div className="card">
          <LoadingRows rows={6} />
        </div>
      </div>
    )
  }

  if (created) {
    return (
      <div className="public-page">
        <div className="card card-pad stack" style={{ textAlign: 'center' }}>
          <div style={{ fontSize: 44 }}>✅</div>
          <h2>Pesanan terkirim!</h2>
          <p className="muted" style={{ margin: 0 }}>
            Tunjukkan nomor pesanan berikut ke kasir untuk pembayaran.
          </p>
          <div style={{ fontSize: 34, fontWeight: 800, letterSpacing: '0.04em' }}>{created.code}</div>
          <div>
            Total tagihan: <strong>{formatRupiah(created.total)}</strong>
          </div>
          <button type="button" className="btn" onClick={() => setCreated(null)}>
            Pesan lagi
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="public-page">
      <header className="public-header">
        <div className="tiny" style={{ opacity: 0.85 }}>
          Pesan online
        </div>
        <h2 style={{ fontSize: 26 }}>{business || 'Menu'}</h2>
        <div className="small" style={{ opacity: 0.9 }}>
          Pilih menu, kirim pesanan, lalu bayar di kasir.
        </div>
      </header>

      <ErrorAlert message={error} />

      {menu.length === 0 ? (
        <div className="card">
          <EmptyState icon="🍽️" title="Menu belum tersedia" hint="Silakan hubungi kasir untuk memesan." />
        </div>
      ) : (
        <form onSubmit={handleSubmit} className="stack">
          <div className="product-grid">
            {menu.map((item) => {
              const qty = cart[item.id] ?? 0
              return (
                <div className="product-card" key={item.id}>
                  {item.image_url ? (
                    <img className="product-thumb" src={item.image_url} alt={item.name} loading="lazy" />
                  ) : (
                    <div className="product-thumb-placeholder" aria-hidden>
                      {item.name.slice(0, 2).toUpperCase()}
                    </div>
                  )}
                  <div className="product-body">
                    <span className="product-name">{item.name}</span>
                    <span className="tiny muted">{item.category}</span>
                    <span className="product-price">{formatRupiah(item.price)}</span>
                    <div className="qty-control" style={{ marginTop: 6, alignSelf: 'flex-start' }}>
                      <button type="button" onClick={() => changeQty(item.id, -1)} aria-label={`Kurangi ${item.name}`}>
                        −
                      </button>
                      <span>{qty}</span>
                      <button type="button" onClick={() => changeQty(item.id, 1)} aria-label={`Tambah ${item.name}`}>
                        +
                      </button>
                    </div>
                  </div>
                </div>
              )
            })}
          </div>

          <div className="card card-pad stack">
            <h3>Data pemesan</h3>
            <div className="field-row">
              <div className="field">
                <label htmlFor="pm-name">Nama</label>
                <input
                  id="pm-name"
                  value={customerName}
                  onChange={(e) => setCustomerName(e.target.value)}
                  placeholder="Nama Anda"
                  required
                />
              </div>
              <div className="field">
                <label htmlFor="pm-table">No. meja (opsional)</label>
                <input id="pm-table" value={tableNo} onChange={(e) => setTableNo(e.target.value)} />
              </div>
            </div>
            <div className="field">
              <label htmlFor="pm-note">Catatan</label>
              <input
                id="pm-note"
                value={note}
                onChange={(e) => setNote(e.target.value)}
                placeholder="Misal: tanpa sambal, es sedikit"
              />
            </div>

            <div className="summary-row">
              <span className="muted">Total</span>
              <span className="summary-total">{formatRupiah(total)}</span>
            </div>
            <button type="submit" className="btn btn-block" disabled={items.length === 0 || submitting}>
              {submitting ? 'Mengirim...' : 'Kirim pesanan'}
            </button>
          </div>
        </form>
      )}
    </div>
  )
}
