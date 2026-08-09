import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { BarChart, EmptyState, ErrorAlert, LoadingRows, StatCard, StatusBadge } from '../components/ui'
import { ApiError, request } from '../lib/api'
import type { Envelope } from '../lib/api'
import { formatNumber, formatRupiah, formatTime } from '../lib/format'
import type { DashboardSummary } from '../lib/types'

/** DashboardPage adalah ringkasan performa toko untuk pemilik. */
export function DashboardPage() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    request<Envelope<DashboardSummary>>('/dashboard')
      .then((res) => {
        if (active) setSummary(res.data)
      })
      .catch((err) => {
        if (active) setError(err instanceof ApiError ? err.message : 'Gagal memuat dashboard')
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [])

  if (loading) {
    return (
      <div className="card">
        <LoadingRows rows={6} />
      </div>
    )
  }

  if (error || !summary) {
    return <ErrorAlert message={error ?? 'Data dashboard tidak tersedia'} />
  }

  const antrian = (summary.orders_by_status.baru ?? 0) + (summary.orders_by_status.diproses ?? 0)

  return (
    <div className="stack">
      <div className="grid grid-4">
        <StatCard
          label="Omzet hari ini"
          value={formatRupiah(summary.today.omzet)}
          hint={`${formatNumber(summary.today.transactions)} transaksi • ${formatNumber(summary.today.items)} item`}
        />
        <StatCard
          label="Omzet bulan ini"
          value={formatRupiah(summary.month.omzet)}
          hint={`Rata-rata belanja ${formatRupiah(summary.month.average_basket)}`}
        />
        <StatCard label="Pesanan aktif" value={formatNumber(antrian)} hint="Status baru + diproses" />
        <StatCard
          label="Produk terdaftar"
          value={formatNumber(summary.product_count)}
          hint={`${summary.low_stock_products.length} produk stok menipis`}
        />
      </div>

      <div className="card">
        <div className="card-header">
          <h3>Tren omzet 7 hari terakhir</h3>
          <span className="badge">harian</span>
        </div>
        <div className="card-pad">
          <BarChart
            data={summary.trend_7_days.map((p) => ({ label: p.label, value: p.omzet }))}
            formatValue={formatRupiah}
          />
        </div>
      </div>

      <div className="grid grid-2">
        <div className="card">
          <div className="card-header">
            <h3>Produk terlaris bulan ini</h3>
            <Link className="small" to="/laporan">
              Lihat laporan
            </Link>
          </div>
          {summary.top_products_month.length === 0 ? (
            <EmptyState title="Belum ada penjualan bulan ini" />
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Produk</th>
                  <th className="table-num">Terjual</th>
                  <th className="table-num">Omzet</th>
                </tr>
              </thead>
              <tbody>
                {summary.top_products_month.map((p) => (
                  <tr key={p.name}>
                    <td>{p.name}</td>
                    <td className="table-num">{formatNumber(p.qty)}</td>
                    <td className="table-num">{formatRupiah(p.omzet)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        <div className="card">
          <div className="card-header">
            <h3>Pesanan berjalan</h3>
            <Link className="small" to="/pesanan">
              Buka papan pesanan
            </Link>
          </div>
          {summary.active_orders.length === 0 ? (
            <EmptyState title="Tidak ada pesanan aktif" hint="Semua pesanan sudah selesai dilayani." />
          ) : (
            <table className="table">
              <thead>
                <tr>
                  <th>Kode</th>
                  <th>Waktu</th>
                  <th>Status</th>
                  <th className="table-num">Total</th>
                </tr>
              </thead>
              <tbody>
                {summary.active_orders.map((order) => (
                  <tr key={order.id}>
                    <td>
                      <div style={{ fontWeight: 600 }}>{order.code}</div>
                      <div className="tiny muted">{order.source === 'online' ? 'Online' : 'Kasir'}</div>
                    </td>
                    <td className="small">{formatTime(order.created_at)}</td>
                    <td>
                      <StatusBadge status={order.status} />
                    </td>
                    <td className="table-num">{formatRupiah(order.total)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>

      <div className="card">
        <div className="card-header">
          <h3>Stok menipis</h3>
          <span className="badge">≤ {summary.low_stock_threshold} unit</span>
        </div>
        {summary.low_stock_products.length === 0 ? (
          <EmptyState title="Semua stok aman" />
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th>Produk</th>
                <th>Kategori</th>
                <th className="table-num">Sisa stok</th>
              </tr>
            </thead>
            <tbody>
              {summary.low_stock_products.map((p) => (
                <tr key={p.id}>
                  <td>{p.name}</td>
                  <td>{p.category || '-'}</td>
                  <td className="table-num">
                    <span className="badge badge-batal">{p.stock}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  )
}
