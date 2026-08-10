import { useEffect, useMemo, useState } from 'react'

import { ApiError, request } from '../lib/api'
import type { Envelope } from '../lib/api'
import { formatRupiah } from '../lib/format'
import type { Customer } from '../lib/types'

/**
 * CustomerPicker membantu kasir menautkan transaksi ke pelanggan terdaftar.
 * Seluruh isian bersifat opsional — transaksi anonim tetap bisa diselesaikan.
 */
export function CustomerPicker({
  selected,
  onSelect,
  name,
  phone,
  onNameChange,
  onPhoneChange,
}: {
  selected: Customer | null
  onSelect: (customer: Customer | null) => void
  name: string
  phone: string
  onNameChange: (value: string) => void
  onPhoneChange: (value: string) => void
}) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<Customer[]>([])
  const [searching, setSearching] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const term = query.trim()
    if (selected || term.length < 2) {
      setResults([])
      return
    }
    // Tunda permintaan agar tiap ketikan tidak memanggil Google Sheets.
    const timer = window.setTimeout(() => {
      setSearching(true)
      request<Envelope<Customer[]>>(`/customers?q=${encodeURIComponent(term)}`)
        .then((res) => setResults((res.data ?? []).slice(0, 5)))
        .catch((err) => setError(err instanceof ApiError ? err.message : 'Pencarian pelanggan gagal'))
        .finally(() => setSearching(false))
    }, 350)
    return () => window.clearTimeout(timer)
  }, [query, selected])

  const summary = useMemo(() => {
    if (!selected) return ''
    return `${selected.points} poin • total belanja ${formatRupiah(selected.total_spent)}`
  }, [selected])

  if (selected) {
    return (
      <div className="picker-selected">
        <span aria-hidden>💳</span>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontWeight: 650 }}>{selected.name}</div>
          <div className="tiny muted">{selected.phone || 'tanpa nomor HP'}</div>
          <div className="tiny" style={{ color: 'var(--brand-600)' }}>
            {summary}
          </div>
        </div>
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          onClick={() => {
            onSelect(null)
            setQuery('')
          }}
        >
          Ganti
        </button>
      </div>
    )
  }

  return (
    <div className="picker">
      <div className="field" style={{ marginBottom: 8 }}>
        <label htmlFor="cust-search">Cari pelanggan terdaftar</label>
        <input
          id="cust-search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Ketik nama atau nomor HP..."
        />
      </div>

      {searching ? <div className="tiny muted">Mencari...</div> : null}
      {error ? <div className="tiny" style={{ color: 'var(--danger)' }}>{error}</div> : null}

      {results.length > 0 ? (
        <div className="suggestions">
          {results.map((customer) => (
            <button
              key={customer.id}
              type="button"
              className="suggestion"
              onClick={() => {
                onSelect(customer)
                setQuery('')
                setResults([])
              }}
            >
              <div style={{ fontWeight: 600 }}>{customer.name}</div>
              <div className="tiny muted">
                {customer.phone || 'tanpa nomor'} • {customer.points} poin
              </div>
            </button>
          ))}
        </div>
      ) : null}

      <div className="field-row" style={{ marginTop: 10 }}>
        <div className="field" style={{ marginBottom: 0 }}>
          <label htmlFor="cust-name">Nama pelanggan</label>
          <input
            id="cust-name"
            value={name}
            onChange={(e) => onNameChange(e.target.value)}
            placeholder="Opsional"
          />
        </div>
        <div className="field" style={{ marginBottom: 0 }}>
          <label htmlFor="cust-phone">No. HP</label>
          <input
            id="cust-phone"
            inputMode="tel"
            value={phone}
            onChange={(e) => onPhoneChange(e.target.value)}
            placeholder="08xx"
          />
        </div>
      </div>
      <div className="hint">
        Isi nomor HP untuk mengumpulkan poin loyalitas. Pelanggan baru otomatis terdaftar; kosongkan saja untuk
        transaksi anonim.
      </div>
    </div>
  )
}
