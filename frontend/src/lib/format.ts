// Helper format angka dan tanggal dalam gaya Indonesia.

const rupiah = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  maximumFractionDigits: 0,
})

const number = new Intl.NumberFormat('id-ID')

/** formatRupiah mengubah angka menjadi "Rp15.000". */
export function formatRupiah(value: number): string {
  return rupiah.format(Math.round(value || 0))
}

/** formatNumber memberi pemisah ribuan. */
export function formatNumber(value: number): string {
  return number.format(value || 0)
}

/** formatDateTime menampilkan tanggal dan jam singkat. */
export function formatDateTime(value: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('id-ID', {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/** formatTime menampilkan jam saja, dipakai pada papan pesanan. */
export function formatTime(value: string): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' })
}

/** toDateInput mengubah Date menjadi nilai <input type="date">. */
export function toDateInput(date: Date): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/** daysAgo mengembalikan tanggal N hari lalu. */
export function daysAgo(days: number): Date {
  const d = new Date()
  d.setDate(d.getDate() - days)
  return d
}

/** initials membuat inisial dari nama untuk avatar sederhana. */
export function initials(name: string): string {
  return name
    .split(' ')
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? '')
    .join('')
}
