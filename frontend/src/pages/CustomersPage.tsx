import { useCallback, useEffect, useState } from 'react'

import { EmptyState, ErrorAlert, LoadingRows, Modal, StatCard, StatusBadge } from '../components/ui'
import { ApiError, request } from '../lib/api'
import type { Envelope } from '../lib/api'
import { formatDateTime, formatNumber, formatRupiah } from '../lib/format'
import type { Customer, CustomerHistory } from '../lib/types'

interface CustomerListResponse {
  data: Customer[]
  meta: { rupiah_per_poin: number }
}

/** CustomersPage adalah modul CRM & loyalitas untuk pemilik. */
export function CustomersPage() {
  const [customers, setCustomers] = useState<Customer[]>([])
  const [rupiahPerPoin, setRupiahPerPoin] = useState(10000)
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Customer | null>(null)
  const [form, setForm] = useState({ name: '', phone: '' })
  const [formError, setFormError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const [history, setHistory] = useState<CustomerHistory | null>(null)
  const [historyLoading, setHistoryLoading] = useState(false)

  const load = useCallback(async () => {
    setError(null)
    try {
      const res = await request<CustomerListResponse>(`/customers?q=${encodeURIComponent(query.trim())}`)
      setCustomers(res.data ?? [])
      if (res.meta?.rupiah_per_poin) setRupiahPerPoin(res.meta.rupiah_per_poin)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal memuat data pelanggan')
    } finally {
      setLoading(false)
    }
  }, [query])

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 300)
    return () => window.clearTimeout(timer)
  }, [load])

  const totalPoints = customers.reduce((sum, c) => sum + c.points, 0)
  const totalSpent = customers.reduce((sum, c) => sum + c.total_spent, 0)

  function openCreate() {
    setEditing(null)
    setForm({ name: '', phone: '' })
    setFormError(null)
    setShowForm(true)
  }

  function openEdit(customer: Customer) {
    setEditing(customer)
    setForm({ name: customer.name, phone: customer.phone })
    setFormError(null)
    setShowForm(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    setFormError(null)
    try {
      if (editing) {
        await request<Envelope<Customer>>(`/customers/${editing.id}`, { method: 'PUT', body: form })
      } else {
        await request<Envelope<Customer>>('/customers', { method: 'POST', body: form })
      }
      setShowForm(false)
      await load()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Gagal menyimpan pelanggan')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(customer: Customer) {
    if (!window.confirm(`Hapus pelanggan "${customer.name}"? Riwayat pesanannya tetap tersimpan.`)) return
    try {
      await request<void>(`/customers/${customer.id}`, { method: 'DELETE' })
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal menghapus pelanggan')
    }
  }

  async function openHistory(customer: Customer) {
    setHistoryLoading(true)
    setHistory({ customer, orders: [] })
    try {
      const res = await request<Envelope<CustomerHistory>>(`/customers/${customer.id}/history`)
      setHistory(res.data)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal memuat riwayat pembelian')
      setHistory(null)
    } finally {
      setHistoryLoading(false)
    }
  }

  return (
    <div className="stack">
      <div className="grid grid-3">
        <StatCard label="Pelanggan terdaftar" value={formatNumber(customers.length)} icon="💳" />
        <StatCard
          label="Total belanja pelanggan"
          value={formatRupiah(totalSpent)}
          hint="Dari seluruh transaksi yang tertaut"
          icon="🧾"
        />
        <StatCard
          label="Poin beredar"
          value={formatNumber(totalPoints)}
          hint={`1 poin per ${formatRupiah(rupiahPerPoin)} belanja`}
          icon="⭐"
        />
      </div>

      <div className="card card-pad">
        <div className="row">
          <input
            style={{ flex: '2 1 240px' }}
            placeholder="Cari nama atau nomor HP..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            aria-label="Cari pelanggan"
          />
          <div className="spacer" />
          <button type="button" className="btn" onClick={openCreate}>
            + Tambah pelanggan
          </button>
        </div>
      </div>

      <ErrorAlert message={error} />

      <div className="card">
        <div className="card-header">
          <h3>Daftar pelanggan</h3>
          <span className="badge">{customers.length} pelanggan</span>
        </div>
        {loading ? (
          <LoadingRows rows={5} />
        ) : customers.length === 0 ? (
          <EmptyState
            icon="💳"
            title={query ? 'Pelanggan tidak ditemukan' : 'Belum ada pelanggan'}
            hint={
              query
                ? 'Coba kata kunci lain, atau daftarkan pelanggan baru.'
                : 'Pelanggan otomatis terdaftar saat kasir mengisi nomor HP pada transaksi.'
            }
            action={
              query ? null : (
                <button type="button" className="btn btn-sm" onClick={openCreate}>
                  Tambah pelanggan
                </button>
              )
            }
          />
        ) : (
          <div className="table-scroll">
            <table className="table">
              <thead>
                <tr>
                  <th>Pelanggan</th>
                  <th>No. HP</th>
                  <th className="table-num">Total belanja</th>
                  <th className="table-num">Poin</th>
                  <th>Terakhir belanja</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {customers.map((customer) => (
                  <tr key={customer.id}>
                    <td>
                      <div style={{ fontWeight: 600 }}>{customer.name}</div>
                      <div className="tiny muted mono">{customer.id}</div>
                    </td>
                    <td className="small mono">{customer.phone || '-'}</td>
                    <td className="table-num">{formatRupiah(customer.total_spent)}</td>
                    <td className="table-num">
                      <span className="badge badge-loyal">{formatNumber(customer.points)}</span>
                    </td>
                    <td className="small muted">
                      {customer.last_purchase ? formatDateTime(customer.last_purchase) : 'belum pernah'}
                    </td>
                    <td>
                      <div className="row row-tight" style={{ justifyContent: 'flex-end', flexWrap: 'nowrap' }}>
                        <button
                          type="button"
                          className="btn btn-secondary btn-sm"
                          onClick={() => void openHistory(customer)}
                        >
                          Riwayat
                        </button>
                        <button type="button" className="btn btn-secondary btn-sm" onClick={() => openEdit(customer)}>
                          Ubah
                        </button>
                        <button
                          type="button"
                          className="btn btn-ghost btn-sm"
                          onClick={() => void handleDelete(customer)}
                        >
                          Hapus
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {showForm ? (
        <Modal
          title={editing ? `Ubah ${editing.name}` : 'Tambah pelanggan'}
          onClose={() => setShowForm(false)}
          footer={
            <>
              <button type="button" className="btn btn-secondary" onClick={() => setShowForm(false)}>
                Batal
              </button>
              <button type="submit" form="customer-form" className="btn" disabled={saving}>
                {saving ? 'Menyimpan...' : 'Simpan'}
              </button>
            </>
          }
        >
          <form id="customer-form" onSubmit={handleSave}>
            <ErrorAlert message={formError} />
            <div className="field" style={{ marginTop: 12 }}>
              <label htmlFor="c-name">Nama pelanggan</label>
              <input
                id="c-name"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                required
              />
            </div>
            <div className="field">
              <label htmlFor="c-phone">Nomor HP</label>
              <input
                id="c-phone"
                inputMode="tel"
                value={form.phone}
                onChange={(e) => setForm({ ...form, phone: e.target.value })}
                placeholder="08xxxxxxxxxx"
              />
              <div className="hint">
                Nomor HP dipakai kasir untuk mengenali pelanggan lama sehingga poinnya terus bertambah.
              </div>
            </div>
          </form>
        </Modal>
      ) : null}

      {history ? (
        <Modal title={`Riwayat ${history.customer.name}`} onClose={() => setHistory(null)} wide>
          <div className="grid grid-3" style={{ marginBottom: 16 }}>
            <StatCard label="Total belanja" value={formatRupiah(history.customer.total_spent)} />
            <StatCard label="Poin" value={formatNumber(history.customer.points)} />
            <StatCard
              label="Jumlah pesanan"
              value={formatNumber(history.orders.length)}
              hint={history.customer.phone || 'tanpa nomor HP'}
            />
          </div>

          {historyLoading ? (
            <LoadingRows rows={4} />
          ) : history.orders.length === 0 ? (
            <EmptyState icon="🧾" title="Belum ada pembelian" hint="Transaksi pelanggan ini akan muncul di sini." />
          ) : (
            <div className="table-scroll">
              <table className="table">
                <thead>
                  <tr>
                    <th>Kode</th>
                    <th>Waktu</th>
                    <th>Item</th>
                    <th>Status</th>
                    <th className="table-num">Total</th>
                  </tr>
                </thead>
                <tbody>
                  {history.orders.map((order) => (
                    <tr key={order.id}>
                      <td className="mono">{order.code}</td>
                      <td className="small">{formatDateTime(order.created_at)}</td>
                      <td className="small muted">
                        {order.items.map((item) => `${item.qty}× ${item.name}`).join(', ')}
                      </td>
                      <td>
                        <StatusBadge status={order.status} />
                      </td>
                      <td className="table-num">{formatRupiah(order.total)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Modal>
      ) : null}
    </div>
  )
}
