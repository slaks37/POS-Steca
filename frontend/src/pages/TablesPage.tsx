import { useCallback, useEffect, useMemo, useState } from 'react'

import { EmptyState, ErrorAlert, LoadingCards, Modal, StatCard, TableStatusBadge } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { ApiError, request } from '../lib/api'
import type { Envelope } from '../lib/api'
import { formatNumber, formatTime } from '../lib/format'
import type { Table, TableStatus } from '../lib/types'

const statusFlow: { value: TableStatus; label: string }[] = [
  { value: 'kosong', label: 'Kosong' },
  { value: 'terisi', label: 'Terisi' },
  { value: 'dibersihkan', label: 'Perlu dibersihkan' },
]

/** TablesPage adalah denah meja: owner mengelola, kasir memantau statusnya. */
export function TablesPage() {
  const { isOwner } = useAuth()
  const [tables, setTables] = useState<Table[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [busyId, setBusyId] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Table | null>(null)
  const [form, setForm] = useState({ name: '', capacity: '4', area: '' })
  const [formError, setFormError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setError(null)
    try {
      const res = await request<Envelope<Table[]>>('/tables')
      setTables(res.data ?? [])
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal memuat denah meja')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const grouped = useMemo(() => {
    const map = new Map<string, Table[]>()
    for (const table of tables) {
      const area = table.area || 'Tanpa area'
      map.set(area, [...(map.get(area) ?? []), table])
    }
    return [...map.entries()]
  }, [tables])

  const counts = useMemo(
    () => ({
      kosong: tables.filter((t) => t.status === 'kosong').length,
      terisi: tables.filter((t) => t.status === 'terisi').length,
      dibersihkan: tables.filter((t) => t.status === 'dibersihkan').length,
    }),
    [tables],
  )

  function openCreate() {
    setEditing(null)
    setForm({ name: '', capacity: '4', area: '' })
    setFormError(null)
    setShowForm(true)
  }

  function openEdit(table: Table) {
    setEditing(table)
    setForm({ name: table.name, capacity: String(table.capacity), area: table.area })
    setFormError(null)
    setShowForm(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    setFormError(null)
    const payload = { name: form.name, capacity: Number(form.capacity) || 0, area: form.area }
    try {
      if (editing) {
        await request<Envelope<Table>>(`/tables/${editing.id}`, { method: 'PUT', body: payload })
      } else {
        await request<Envelope<Table>>('/tables', { method: 'POST', body: payload })
      }
      setShowForm(false)
      await load()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Gagal menyimpan meja')
    } finally {
      setSaving(false)
    }
  }

  async function changeStatus(table: Table, status: TableStatus) {
    setBusyId(table.id)
    setError(null)
    try {
      await request<Envelope<Table>>(`/tables/${table.id}/status`, { method: 'PATCH', body: { status } })
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal mengubah status meja')
    } finally {
      setBusyId(null)
    }
  }

  async function handleDelete(table: Table) {
    if (!window.confirm(`Hapus ${table.name}?`)) return
    try {
      await request<void>(`/tables/${table.id}`, { method: 'DELETE' })
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal menghapus meja')
    }
  }

  return (
    <div className="stack">
      <div className="grid grid-3">
        <StatCard label="Meja kosong" value={formatNumber(counts.kosong)} icon="✅" />
        <StatCard label="Meja terisi" value={formatNumber(counts.terisi)} icon="🍽️" />
        <StatCard label="Perlu dibersihkan" value={formatNumber(counts.dibersihkan)} icon="🧹" />
      </div>

      {isOwner ? (
        <div className="card card-pad">
          <div className="row">
            <div className="small muted">
              Atur nomor/nama meja beserta kapasitasnya. Meja yang dipilih di kasir otomatis berstatus terisi.
            </div>
            <div className="spacer" />
            <button type="button" className="btn" onClick={openCreate}>
              + Tambah meja
            </button>
          </div>
        </div>
      ) : null}

      <ErrorAlert message={error} />

      {loading ? (
        <LoadingCards count={6} tile />
      ) : tables.length === 0 ? (
        <div className="card">
          <EmptyState
            icon="🪑"
            title="Belum ada meja"
            hint={
              isOwner
                ? 'Tambahkan meja agar kasir bisa mengaitkan pesanan ke nomor meja.'
                : 'Minta pemilik menambahkan denah meja terlebih dahulu.'
            }
            action={
              isOwner ? (
                <button type="button" className="btn btn-sm" onClick={openCreate}>
                  Tambah meja
                </button>
              ) : null
            }
          />
        </div>
      ) : (
        grouped.map(([area, items]) => (
          <div className="stack" key={area} style={{ gap: 12 }}>
            <div className="row">
              <strong>{area}</strong>
              <span className="badge">{items.length} meja</span>
            </div>
            <div className="floor-grid">
              {items.map((table) => (
                <div className={`floor-tile is-${table.status}`} key={table.id}>
                  <div className="row" style={{ justifyContent: 'space-between' }}>
                    <span className="floor-name">{table.name}</span>
                    <TableStatusBadge status={table.status} />
                  </div>
                  <div className="tiny muted">
                    {table.capacity > 0 ? `${table.capacity} kursi` : 'kapasitas belum diatur'}
                    {table.updated_at ? ` • diperbarui ${formatTime(table.updated_at)}` : ''}
                  </div>

                  <div className="row row-tight">
                    {statusFlow
                      .filter((s) => s.value !== table.status)
                      .map((s) => (
                        <button
                          key={s.value}
                          type="button"
                          className="btn btn-secondary btn-sm"
                          disabled={busyId === table.id}
                          onClick={() => void changeStatus(table, s.value)}
                        >
                          {s.label}
                        </button>
                      ))}
                  </div>

                  {isOwner ? (
                    <div className="row row-tight" style={{ marginTop: 'auto' }}>
                      <button type="button" className="btn btn-ghost btn-sm" onClick={() => openEdit(table)}>
                        Ubah
                      </button>
                      <button type="button" className="btn btn-ghost btn-sm" onClick={() => void handleDelete(table)}>
                        Hapus
                      </button>
                    </div>
                  ) : null}
                </div>
              ))}
            </div>
          </div>
        ))
      )}

      {showForm ? (
        <Modal
          title={editing ? `Ubah ${editing.name}` : 'Tambah meja'}
          onClose={() => setShowForm(false)}
          footer={
            <>
              <button type="button" className="btn btn-secondary" onClick={() => setShowForm(false)}>
                Batal
              </button>
              <button type="submit" form="table-form" className="btn" disabled={saving}>
                {saving ? 'Menyimpan...' : 'Simpan'}
              </button>
            </>
          }
        >
          <form id="table-form" onSubmit={handleSave}>
            <ErrorAlert message={formError} />
            <div className="field" style={{ marginTop: 12 }}>
              <label htmlFor="t-name">Nomor / nama meja</label>
              <input
                id="t-name"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="Meja 1"
                required
              />
            </div>
            <div className="field-row">
              <div className="field">
                <label htmlFor="t-capacity">Kapasitas (kursi)</label>
                <input
                  id="t-capacity"
                  inputMode="numeric"
                  value={form.capacity}
                  onChange={(e) => setForm({ ...form, capacity: e.target.value })}
                />
              </div>
              <div className="field">
                <label htmlFor="t-area">Area</label>
                <input
                  id="t-area"
                  value={form.area}
                  onChange={(e) => setForm({ ...form, area: e.target.value })}
                  placeholder="Indoor / Outdoor"
                />
              </div>
            </div>
          </form>
        </Modal>
      ) : null}
    </div>
  )
}
