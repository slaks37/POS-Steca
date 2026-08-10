import { useCallback, useEffect, useMemo, useState } from 'react'

import { ReceiptView } from '../components/ReceiptView'
import { EmptyState, ErrorAlert, LoadingCards, Modal, StatusBadge } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { ApiError, request } from '../lib/api'
import type { Envelope } from '../lib/api'
import { formatRupiah, formatTime } from '../lib/format'
import type { CheckoutResult, Order, OrderSource, OrderStatus, PaymentMethod } from '../lib/types'

const columns: { status: OrderStatus; title: string; next?: OrderStatus; nextLabel?: string }[] = [
  { status: 'baru', title: 'Baru', next: 'diproses', nextLabel: 'Proses' },
  { status: 'diproses', title: 'Diproses', next: 'selesai', nextLabel: 'Selesaikan' },
  { status: 'selesai', title: 'Selesai' },
]

const sourceLabels: Record<OrderSource, string> = {
  kasir: 'Kasir langsung',
  online: 'Pesanan online',
}

/** OrdersPage adalah papan manajemen pesanan ala Trofi. */
export function OrdersPage() {
  const { tenant } = useAuth()
  const [orders, setOrders] = useState<Order[]>([])
  const [sourceFilter, setSourceFilter] = useState<'' | OrderSource>('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)
  const [settling, setSettling] = useState<Order | null>(null)
  const [settlePayment, setSettlePayment] = useState<PaymentMethod>('tunai')
  const [settlePhone, setSettlePhone] = useState('')
  const [receipt, setReceipt] = useState<CheckoutResult | null>(null)

  const load = useCallback(async () => {
    setError(null)
    try {
      const query = sourceFilter ? `?source=${sourceFilter}` : ''
      const res = await request<Envelope<Order[]>>(`/orders${query}`)
      setOrders(res.data ?? [])
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal memuat pesanan')
    } finally {
      setLoading(false)
    }
  }, [sourceFilter])

  useEffect(() => {
    setLoading(true)
    void load()
  }, [load])

  useEffect(() => {
    // Papan pesanan dapur perlu terus segar; muat ulang berkala.
    const timer = window.setInterval(() => void load(), 30_000)
    return () => window.clearInterval(timer)
  }, [load])

  const grouped = useMemo(() => {
    const map: Record<OrderStatus, Order[]> = { baru: [], diproses: [], selesai: [], batal: [] }
    for (const order of orders) map[order.status]?.push(order)
    return map
  }, [orders])

  async function moveStatus(order: Order, status: OrderStatus) {
    setBusyId(order.id)
    setError(null)
    try {
      await request<Envelope<Order>>(`/orders/${order.id}/status`, { method: 'PATCH', body: { status } })
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal mengubah status pesanan')
    } finally {
      setBusyId(null)
    }
  }

  async function handleSettle() {
    if (!settling) return
    setBusyId(settling.id)
    setError(null)
    try {
      const res = await request<Envelope<CheckoutResult>>(`/orders/${settling.id}/settle`, {
        method: 'POST',
        body: {
          payment_method: settlePayment,
          amount_paid: settling.total,
          customer_phone: settlePhone,
        },
      })
      setSettling(null)
      setSettlePhone('')
      setReceipt(res.data)
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal menyelesaikan pembayaran')
    } finally {
      setBusyId(null)
    }
  }

  if (loading) {
    return (
      <div className="stack">
        <div className="skeleton skeleton-card" />
        <LoadingCards count={3} tile />
      </div>
    )
  }

  return (
    <div className="stack">
      <div className="card card-pad">
        <div className="row">
          <strong>Sumber pesanan</strong>
          <select
            style={{ width: 200 }}
            value={sourceFilter}
            onChange={(e) => setSourceFilter(e.target.value as '' | OrderSource)}
            aria-label="Filter sumber pesanan"
          >
            <option value="">Semua sumber</option>
            <option value="kasir">Kasir langsung</option>
            <option value="online">Pesanan online</option>
          </select>
          <div className="spacer" />
          <span className="small muted">
            Link pesan online: <code>/menu/{tenant?.code}</code>
          </span>
          <button type="button" className="btn btn-secondary btn-sm" onClick={() => void load()}>
            Muat ulang
          </button>
        </div>
      </div>

      <ErrorAlert message={error} />

      <div className="board">
        {columns.map((col) => (
          <section className="board-col" key={col.status}>
            <header className="board-col-head">
              <span>{col.title}</span>
              <span className="badge">{grouped[col.status].length}</span>
            </header>

            {grouped[col.status].length === 0 ? (
              <EmptyState icon="🍽️" title="Tidak ada pesanan" hint="Pesanan baru akan muncul di sini." />
            ) : (
              grouped[col.status].map((order) => (
                <article className="order-card" key={order.id}>
                  <div className="order-card-head">
                    <span className="order-code">{order.code}</span>
                    <StatusBadge status={order.status} />
                  </div>
                  <div className="tiny muted">
                    {formatTime(order.created_at)} • {sourceLabels[order.source] ?? order.source}
                  </div>
                  <div className="row row-tight" style={{ marginTop: 4 }}>
                    {order.table_no ? <span className="badge badge-terisi">🪑 {order.table_no}</span> : null}
                    {order.customer_name ? (
                      <span className={`badge${order.customer_id ? ' badge-loyal' : ''}`}>
                        {order.customer_id ? '💳' : '👤'} {order.customer_name}
                      </span>
                    ) : null}
                  </div>

                  <ul className="order-items" style={{ paddingLeft: 18, margin: '8px 0' }}>
                    {order.items.map((item, i) => (
                      <li key={`${item.product_id}-${i}`}>
                        {item.qty} × {item.name}
                      </li>
                    ))}
                  </ul>
                  {order.note ? <div className="tiny" style={{ color: 'var(--accent)' }}>📝 {order.note}</div> : null}

                  <div className="row" style={{ justifyContent: 'space-between', marginTop: 10 }}>
                    <strong>{formatRupiah(order.total)}</strong>
                    <span className={`badge ${order.transaction_id ? 'badge-selesai' : ''}`}>
                      {order.transaction_id ? 'Lunas' : 'Belum bayar'}
                    </span>
                  </div>

                  <div className="row" style={{ marginTop: 10 }}>
                    {!order.transaction_id ? (
                      <button
                        type="button"
                        className="btn btn-sm"
                        disabled={busyId === order.id}
                        onClick={() => {
                          setSettling(order)
                          setSettlePayment('tunai')
                          setSettlePhone('')
                        }}
                      >
                        Terima bayar
                      </button>
                    ) : null}
                    {col.next ? (
                      <button
                        type="button"
                        className="btn btn-secondary btn-sm"
                        disabled={busyId === order.id}
                        onClick={() => void moveStatus(order, col.next as OrderStatus)}
                      >
                        {col.nextLabel}
                      </button>
                    ) : null}
                    {!order.transaction_id && order.status !== 'selesai' ? (
                      <button
                        type="button"
                        className="btn btn-ghost btn-sm"
                        disabled={busyId === order.id}
                        onClick={() => void moveStatus(order, 'batal')}
                      >
                        Batalkan
                      </button>
                    ) : null}
                  </div>
                </article>
              ))
            )}
          </section>
        ))}
      </div>

      {grouped.batal.length > 0 ? (
        <div className="card">
          <div className="card-header">
            <h3>Pesanan dibatalkan</h3>
            <span className="badge">{grouped.batal.length}</span>
          </div>
          <div className="table-scroll">
            <table className="table">
              <thead>
                <tr>
                  <th>Kode</th>
                  <th>Waktu</th>
                  <th>Pelanggan</th>
                  <th className="table-num">Total</th>
                </tr>
              </thead>
              <tbody>
                {grouped.batal.map((order) => (
                  <tr key={order.id}>
                    <td>{order.code}</td>
                    <td>{formatTime(order.created_at)}</td>
                    <td>{order.customer_name || '-'}</td>
                    <td className="table-num">{formatRupiah(order.total)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ) : null}

      {settling ? (
        <Modal
          title={`Terima pembayaran ${settling.code}`}
          onClose={() => setSettling(null)}
          footer={
            <>
              <button type="button" className="btn btn-secondary" onClick={() => setSettling(null)}>
                Batal
              </button>
              <button type="button" className="btn" disabled={busyId === settling.id} onClick={() => void handleSettle()}>
                Konfirmasi {formatRupiah(settling.total)}
              </button>
            </>
          }
        >
          <div className="stack">
            <div className="summary-row">
              <span className="muted">Total tagihan</span>
              <span className="summary-total">{formatRupiah(settling.total)}</span>
            </div>
            <div>
              <label>Metode pembayaran</label>
              <div className="choice-grid">
                {(['tunai', 'qris', 'kartu'] as PaymentMethod[]).map((method) => (
                  <button
                    type="button"
                    key={method}
                    className={`choice${settlePayment === method ? ' active' : ''}`}
                    onClick={() => setSettlePayment(method)}
                  >
                    {method.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>
            {settling.customer_id ? null : (
              <div className="field" style={{ marginBottom: 0 }}>
                <label htmlFor="settle-phone">No. HP pelanggan (opsional)</label>
                <input
                  id="settle-phone"
                  inputMode="tel"
                  value={settlePhone}
                  onChange={(e) => setSettlePhone(e.target.value)}
                  placeholder="08xxxxxxxxxx"
                />
                <div className="hint">Isi untuk mengumpulkan poin loyalitas atas pesanan ini.</div>
              </div>
            )}
          </div>
        </Modal>
      ) : null}

      {receipt?.receipt ? (
        <Modal
          title="Pembayaran diterima"
          onClose={() => setReceipt(null)}
          footer={
            <>
              <button type="button" className="btn btn-secondary" onClick={() => window.print()}>
                Cetak struk
              </button>
              <button type="button" className="btn" onClick={() => setReceipt(null)}>
                Selesai
              </button>
            </>
          }
        >
          <ReceiptView
            receipt={receipt.receipt}
            order={receipt.order}
            businessName={tenant?.business_name ?? 'Steca POS'}
          />
        </Modal>
      ) : null}
    </div>
  )
}
