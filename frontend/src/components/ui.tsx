import type { ReactNode } from 'react'

import type { OrderStatus } from '../lib/types'

/** Modal sederhana dengan backdrop yang bisa diklik untuk menutup. */
export function Modal({
  title,
  onClose,
  children,
  footer,
}: {
  title: string
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
}) {
  return (
    <div className="modal-backdrop" onClick={onClose} role="presentation">
      <div className="modal" onClick={(e) => e.stopPropagation()} role="dialog" aria-modal="true" aria-label={title}>
        <div className="modal-head">
          <h3>{title}</h3>
          <button type="button" className="btn btn-ghost btn-sm" onClick={onClose} aria-label="Tutup">
            ✕
          </button>
        </div>
        <div className="modal-body">{children}</div>
        {footer ? <div className="modal-foot">{footer}</div> : null}
      </div>
    </div>
  )
}

/** StatCard menampilkan satu angka ringkasan. */
export function StatCard({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="card card-pad">
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
      {hint ? <div className="tiny muted">{hint}</div> : null}
    </div>
  )
}

const statusLabels: Record<OrderStatus, string> = {
  baru: 'Baru',
  diproses: 'Diproses',
  selesai: 'Selesai',
  batal: 'Batal',
}

/** StatusBadge memberi warna berbeda per status pesanan. */
export function StatusBadge({ status }: { status: OrderStatus }) {
  return <span className={`badge badge-${status}`}>{statusLabels[status] ?? status}</span>
}

/** ErrorAlert menampilkan pesan galat dari backend. */
export function ErrorAlert({ message }: { message?: string | null }) {
  if (!message) return null
  return (
    <div className="alert alert-error" role="alert">
      {message}
    </div>
  )
}

/** EmptyState dipakai saat daftar kosong. */
export function EmptyState({ title, hint }: { title: string; hint?: string }) {
  return (
    <div className="empty">
      <div style={{ fontWeight: 600, marginBottom: 4 }}>{title}</div>
      {hint ? <div className="small">{hint}</div> : null}
    </div>
  )
}

/** LoadingRows adalah placeholder saat data sedang dimuat. */
export function LoadingRows({ rows = 4 }: { rows?: number }) {
  return (
    <div className="stack card-pad" style={{ gap: 10 }}>
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="skeleton" style={{ width: `${90 - i * 12}%` }} />
      ))}
    </div>
  )
}

/** BarChart menggambar grafik batang ringan tanpa dependensi eksternal. */
export function BarChart({
  data,
  formatValue,
}: {
  data: { label: string; value: number }[]
  formatValue: (value: number) => string
}) {
  const max = Math.max(...data.map((d) => d.value), 1)
  return (
    <div className="bars">
      {data.map((d, i) => (
        <div className="bar-col" key={`${d.label}-${i}`} title={`${d.label}: ${formatValue(d.value)}`}>
          <div className="tiny muted">{d.value > 0 ? formatValue(d.value) : ''}</div>
          <div className="bar" style={{ height: `${Math.max((d.value / max) * 100, 2)}%` }} />
          <div className="bar-label">{d.label}</div>
        </div>
      ))}
    </div>
  )
}
