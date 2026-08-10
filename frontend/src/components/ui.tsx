import type { ReactNode } from 'react'

import type { OrderStatus, TableStatus } from '../lib/types'

/** Brand menampilkan wordmark "Steca POS" dengan logo teks sederhana. */
export function Brand({ size = 'md', tagline }: { size?: 'md' | 'lg'; tagline?: string }) {
  return (
    <span className="brand">
      <span className={`brand-mark${size === 'lg' ? ' brand-mark-lg' : ''}`} aria-hidden>
        S
      </span>
      <span className="brand-text">
        <span className="brand-name">
          Steca<em>POS</em>
        </span>
        {tagline ? <span className="brand-tagline">{tagline}</span> : null}
      </span>
    </span>
  )
}

/** Modal dengan backdrop yang bisa diklik untuk menutup. */
export function Modal({
  title,
  onClose,
  children,
  footer,
  wide = false,
}: {
  title: string
  onClose: () => void
  children: ReactNode
  footer?: ReactNode
  wide?: boolean
}) {
  return (
    <div className="modal-backdrop" onClick={onClose} role="presentation">
      <div
        className={`modal${wide ? ' modal-wide' : ''}`}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
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
export function StatCard({
  label,
  value,
  hint,
  icon,
}: {
  label: string
  value: string
  hint?: string
  icon?: string
}) {
  return (
    <div className="card card-pad stat-card">
      {icon ? (
        <span className="stat-icon" aria-hidden>
          {icon}
        </span>
      ) : null}
      <div className="stat-label">{label}</div>
      <div className="stat-value">{value}</div>
      {hint ? <div className="tiny muted">{hint}</div> : null}
    </div>
  )
}

const orderStatusLabels: Record<OrderStatus, string> = {
  baru: 'Baru',
  diproses: 'Diproses',
  selesai: 'Selesai',
  batal: 'Batal',
}

/** StatusBadge memberi warna berbeda per status pesanan. */
export function StatusBadge({ status }: { status: OrderStatus }) {
  return <span className={`badge badge-${status}`}>{orderStatusLabels[status] ?? status}</span>
}

const tableStatusLabels: Record<TableStatus, string> = {
  kosong: 'Kosong',
  terisi: 'Terisi',
  dibersihkan: 'Perlu dibersihkan',
}

/** TableStatusBadge menampilkan kondisi meja. */
export function TableStatusBadge({ status }: { status: TableStatus }) {
  return <span className={`badge badge-${status}`}>{tableStatusLabels[status] ?? status}</span>
}

/** tableStatusLabel dipakai di luar badge, misal pada tombol pemilihan meja. */
export function tableStatusLabel(status: TableStatus): string {
  return tableStatusLabels[status] ?? status
}

/** ErrorAlert menampilkan pesan galat dari backend. */
export function ErrorAlert({ message }: { message?: string | null }) {
  if (!message) return null
  return (
    <div className="alert alert-error" role="alert">
      <span aria-hidden>⚠️</span>
      <span>{message}</span>
    </div>
  )
}

/** EmptyState dipakai saat daftar kosong, lengkap dengan ikon dan aksi. */
export function EmptyState({
  icon = '📭',
  title,
  hint,
  action,
}: {
  icon?: string
  title: string
  hint?: string
  action?: ReactNode
}) {
  return (
    <div className="empty">
      <span className="empty-icon" aria-hidden>
        {icon}
      </span>
      <span className="empty-title">{title}</span>
      {hint ? <span className="empty-hint">{hint}</span> : null}
      {action ? <div style={{ marginTop: 8 }}>{action}</div> : null}
    </div>
  )
}

/** LoadingRows adalah placeholder baris tabel saat data dimuat. */
export function LoadingRows({ rows = 4 }: { rows?: number }) {
  return (
    <div className="stack card-pad" style={{ gap: 12 }} aria-busy="true" aria-label="Memuat data">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="skeleton" style={{ width: `${92 - i * 11}%` }} />
      ))}
    </div>
  )
}

/** LoadingCards meniru bentuk kartu statistik saat dimuat. */
export function LoadingCards({ count = 4, tile = false }: { count?: number; tile?: boolean }) {
  return (
    <div className={`grid ${tile ? 'grid-3' : 'grid-4'}`} aria-busy="true" aria-label="Memuat data">
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className={`skeleton ${tile ? 'skeleton-tile' : 'skeleton-card'}`} />
      ))}
    </div>
  )
}

/** Toast adalah notifikasi singkat untuk aksi yang berhasil. */
export function Toast({ message }: { message: string }) {
  return (
    <div className="toast" role="status">
      {message}
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
          <div className="tiny muted num">{d.value > 0 ? formatValue(d.value) : ''}</div>
          <div
            className="bar"
            style={{ height: `${Math.max((d.value / max) * 100, 2)}%`, animationDelay: `${i * 40}ms` }}
          />
          <div className="bar-label">{d.label}</div>
        </div>
      ))}
    </div>
  )
}
