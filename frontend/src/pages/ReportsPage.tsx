import { useCallback, useEffect, useState } from 'react'

import { BarChart, EmptyState, ErrorAlert, LoadingCards, StatCard } from '../components/ui'
import { ApiError, request } from '../lib/api'
import type { Envelope } from '../lib/api'
import { daysAgo, formatDateTime, formatNumber, formatRupiah, toDateInput } from '../lib/format'
import type { SalesReport, Transaction } from '../lib/types'

type Granularity = 'harian' | 'mingguan' | 'bulanan'

const granularities: { value: Granularity; label: string }[] = [
  { value: 'harian', label: 'Harian' },
  { value: 'mingguan', label: 'Mingguan' },
  { value: 'bulanan', label: 'Bulanan' },
]

const paymentLabels: Record<string, string> = {
  tunai: 'Tunai',
  qris: 'QRIS',
  kartu: 'Kartu',
  lainnya: 'Lainnya',
}

/** ReportsPage membaca sheet "Transactions" dan menyajikannya sebagai laporan. */
export function ReportsPage() {
  const [granularity, setGranularity] = useState<Granularity>('harian')
  const [from, setFrom] = useState(toDateInput(daysAgo(29)))
  const [to, setTo] = useState(toDateInput(new Date()))
  const [report, setReport] = useState<SalesReport | null>(null)
  const [transactions, setTransactions] = useState<Transaction[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    const range = `from=${from}&to=${to}`
    try {
      const [salesRes, trxRes] = await Promise.all([
        request<Envelope<SalesReport>>(`/reports/sales?granularity=${granularity}&${range}`),
        request<Envelope<Transaction[]>>(`/reports/transactions?${range}&limit=50`),
      ])
      setReport(salesRes.data)
      setTransactions(trxRes.data ?? [])
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal memuat laporan')
    } finally {
      setLoading(false)
    }
  }, [granularity, from, to])

  useEffect(() => {
    void load()
  }, [load])

  return (
    <div className="stack">
      <div className="card card-pad">
        <div className="row">
          <div>
            <label htmlFor="from">Dari tanggal</label>
            <input id="from" type="date" value={from} max={to} onChange={(e) => setFrom(e.target.value)} />
          </div>
          <div>
            <label htmlFor="to">Sampai tanggal</label>
            <input id="to" type="date" value={to} min={from} onChange={(e) => setTo(e.target.value)} />
          </div>
          <div>
            <label htmlFor="granularity">Kelompokkan per</label>
            <select
              id="granularity"
              value={granularity}
              onChange={(e) => setGranularity(e.target.value as Granularity)}
            >
              {granularities.map((g) => (
                <option key={g.value} value={g.value}>
                  {g.label}
                </option>
              ))}
            </select>
          </div>
          <div className="spacer" />
          <button type="button" className="btn btn-secondary" onClick={() => void load()} disabled={loading}>
            {loading ? 'Memuat...' : 'Terapkan'}
          </button>
        </div>
      </div>

      <ErrorAlert message={error} />

      {loading ? (
        <div className="stack">
          <LoadingCards count={4} />
          <div className="skeleton" style={{ height: 260, borderRadius: 12 }} />
        </div>
      ) : report ? (
        <>
          <div className="grid grid-4">
            <StatCard icon="💰" label="Total omzet" value={formatRupiah(report.total_omzet)} />
            <StatCard icon="🧾" label="Jumlah transaksi" value={formatNumber(report.total_transactions)} />
            <StatCard icon="📦" label="Item terjual" value={formatNumber(report.total_items)} />
            <StatCard icon="📊" label="Rata-rata belanja" value={formatRupiah(report.average_basket)} />
          </div>

          <div className="card">
            <div className="card-header">
              <h3>Grafik omzet</h3>
              <span className="badge">{report.granularity}</span>
            </div>
            <div className="card-pad">
              {report.series.length === 0 ? (
                <EmptyState icon="📊" title="Belum ada penjualan pada rentang ini" hint="Coba perlebar rentang tanggalnya." />
              ) : (
                <BarChart
                  data={report.series.slice(-14).map((p) => ({ label: p.label, value: p.omzet }))}
                  formatValue={formatRupiah}
                />
              )}
            </div>
          </div>

          <div className="grid grid-2">
            <div className="card">
              <div className="card-header">
                <h3>Produk terlaris</h3>
              </div>
              {report.top_products.length === 0 ? (
                <EmptyState icon="📄" title="Belum ada data" />
              ) : (
                <table className="table">
                  <thead>
                    <tr>
                      <th>#</th>
                      <th>Produk</th>
                      <th className="table-num">Terjual</th>
                      <th className="table-num">Omzet</th>
                    </tr>
                  </thead>
                  <tbody>
                    {report.top_products.slice(0, 10).map((p, i) => (
                      <tr key={p.name}>
                        <td>
                          <span className={`rank${i === 0 ? ' rank-1' : ''}`}>{i + 1}</span>
                        </td>
                        <td>{p.name}</td>
                        <td className="table-num">{formatNumber(p.qty)}</td>
                        <td className="table-num">{formatRupiah(p.omzet)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>

            <div className="stack">
              <div className="card">
                <div className="card-header">
                  <h3>Metode pembayaran</h3>
                </div>
                {report.payments.length === 0 ? (
                  <EmptyState icon="📄" title="Belum ada data" />
                ) : (
                  <table className="table">
                    <thead>
                      <tr>
                        <th>Metode</th>
                        <th className="table-num">Transaksi</th>
                        <th className="table-num">Omzet</th>
                      </tr>
                    </thead>
                    <tbody>
                      {report.payments.map((p) => (
                        <tr key={p.method}>
                          <td>{paymentLabels[p.method] ?? p.method}</td>
                          <td className="table-num">{formatNumber(p.transactions)}</td>
                          <td className="table-num">{formatRupiah(p.omzet)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>

              <div className="card">
                <div className="card-header">
                  <h3>Performa kasir</h3>
                </div>
                {report.cashiers.length === 0 ? (
                  <EmptyState icon="📄" title="Belum ada data" />
                ) : (
                  <table className="table">
                    <thead>
                      <tr>
                        <th>Kasir</th>
                        <th className="table-num">Transaksi</th>
                        <th className="table-num">Omzet</th>
                      </tr>
                    </thead>
                    <tbody>
                      {report.cashiers.map((c) => (
                        <tr key={c.cashier}>
                          <td>{c.cashier}</td>
                          <td className="table-num">{formatNumber(c.transactions)}</td>
                          <td className="table-num">{formatRupiah(c.omzet)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
          </div>

          <div className="card">
            <div className="card-header">
              <h3>Riwayat transaksi</h3>
              <span className="badge">{transactions.length} struk terakhir</span>
            </div>
            {transactions.length === 0 ? (
              <EmptyState icon="🧾" title="Belum ada transaksi" />
            ) : (
              <div className="table-scroll">
                <table className="table">
                  <thead>
                    <tr>
                      <th>ID Transaksi</th>
                      <th>Waktu</th>
                      <th>Item</th>
                      <th>Metode</th>
                      <th>Kasir</th>
                      <th className="table-num">Total</th>
                    </tr>
                  </thead>
                  <tbody>
                    {transactions.map((trx) => (
                      <tr key={trx.id}>
                        <td className="small">{trx.id}</td>
                        <td className="small">{formatDateTime(trx.date)}</td>
                        <td className="small muted">
                          {trx.items.map((item) => `${item.qty}× ${item.name}`).join(', ')}
                        </td>
                        <td>{paymentLabels[trx.payment_method] ?? trx.payment_method}</td>
                        <td className="small">{trx.cashier || '-'}</td>
                        <td className="table-num">{formatRupiah(trx.total)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </>
      ) : null}
    </div>
  )
}
