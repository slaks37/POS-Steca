import { useCallback, useEffect, useState } from 'react'

import { EmptyState, ErrorAlert, LoadingRows, Modal } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { ApiError, request } from '../lib/api'
import type { Envelope } from '../lib/api'
import { formatDateTime } from '../lib/format'
import type { Employee, Role } from '../lib/types'

interface FormState {
  name: string
  email: string
  role: Role
  pin: string
  active: boolean
}

const emptyForm: FormState = { name: '', email: '', role: 'kasir', pin: '', active: true }

const roleDescriptions: Record<Role, string> = {
  owner: 'Akses penuh: dashboard, laporan, produk, karyawan, dan kasir.',
  kasir: 'Akses kasir, papan pesanan, dan melihat katalog produk saja.',
}

/** EmployeesPage mengelola karyawan dan level aksesnya (khusus owner). */
export function EmployeesPage() {
  const { tenant } = useAuth()
  const [employees, setEmployees] = useState<Employee[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [showForm, setShowForm] = useState(false)
  const [editing, setEditing] = useState<Employee | null>(null)
  const [form, setForm] = useState<FormState>(emptyForm)
  const [formError, setFormError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const load = useCallback(async () => {
    setError(null)
    try {
      const res = await request<Envelope<Employee[]>>('/employees')
      setEmployees(res.data ?? [])
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal memuat data karyawan')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  function openCreate() {
    setEditing(null)
    setForm(emptyForm)
    setFormError(null)
    setShowForm(true)
  }

  function openEdit(employee: Employee) {
    setEditing(employee)
    setForm({ name: employee.name, email: employee.email, role: employee.role, pin: '', active: employee.active })
    setFormError(null)
    setShowForm(true)
  }

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    setFormError(null)
    try {
      if (editing) {
        await request<Envelope<Employee>>(`/employees/${editing.id}`, {
          method: 'PUT',
          body: { name: form.name, role: form.role, active: form.active, pin: form.pin || undefined },
        })
      } else {
        await request<Envelope<Employee>>('/employees', {
          method: 'POST',
          body: { name: form.name, email: form.email, role: form.role, pin: form.pin, active: form.active },
        })
      }
      setShowForm(false)
      await load()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : 'Gagal menyimpan karyawan')
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(employee: Employee) {
    if (!window.confirm(`Hapus karyawan "${employee.name}"?`)) return
    setError(null)
    try {
      await request<void>(`/employees/${employee.id}`, { method: 'DELETE' })
      await load()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal menghapus karyawan')
    }
  }

  return (
    <div className="stack">
      <div className="alert alert-info">
        Karyawan masuk lewat halaman login dengan <strong>kode bisnis {tenant?.code}</strong>, email mereka, dan PIN
        yang Anda buat di sini.
      </div>

      <div className="card">
        <div className="card-header">
          <h3>Daftar karyawan</h3>
          <button type="button" className="btn btn-sm" onClick={openCreate}>
            + Tambah karyawan
          </button>
        </div>

        <ErrorAlert message={error} />

        {loading ? (
          <LoadingRows rows={4} />
        ) : employees.length === 0 ? (
          <EmptyState title="Belum ada karyawan" />
        ) : (
          <div className="table-scroll">
            <table className="table">
              <thead>
                <tr>
                  <th>Nama</th>
                  <th>Email</th>
                  <th>Role</th>
                  <th>Status</th>
                  <th>Bergabung</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {employees.map((employee) => {
                  const isOwnerAccount = employee.email === tenant?.owner_email
                  return (
                    <tr key={employee.id}>
                      <td style={{ fontWeight: 600 }}>{employee.name}</td>
                      <td className="small muted">{employee.email}</td>
                      <td>
                        <span className={`badge ${employee.role === 'owner' ? 'badge-selesai' : 'badge-baru'}`}>
                          {employee.role}
                        </span>
                      </td>
                      <td>
                        <span className={`badge ${employee.active ? '' : 'badge-batal'}`}>
                          {employee.active ? 'Aktif' : 'Nonaktif'}
                        </span>
                      </td>
                      <td className="small muted">{formatDateTime(employee.created_at)}</td>
                      <td>
                        <div className="row" style={{ justifyContent: 'flex-end', flexWrap: 'nowrap' }}>
                          <button type="button" className="btn btn-secondary btn-sm" onClick={() => openEdit(employee)}>
                            Ubah
                          </button>
                          {!isOwnerAccount ? (
                            <button
                              type="button"
                              className="btn btn-ghost btn-sm"
                              onClick={() => void handleDelete(employee)}
                            >
                              Hapus
                            </button>
                          ) : null}
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {showForm ? (
        <Modal
          title={editing ? `Ubah ${editing.name}` : 'Tambah karyawan'}
          onClose={() => setShowForm(false)}
          footer={
            <>
              <button type="button" className="btn btn-secondary" onClick={() => setShowForm(false)}>
                Batal
              </button>
              <button type="submit" form="employee-form" className="btn" disabled={saving}>
                {saving ? 'Menyimpan...' : 'Simpan'}
              </button>
            </>
          }
        >
          <form id="employee-form" onSubmit={handleSave}>
            <ErrorAlert message={formError} />
            <div className="field" style={{ marginTop: 12 }}>
              <label htmlFor="e-name">Nama lengkap</label>
              <input id="e-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
            </div>
            <div className="field">
              <label htmlFor="e-email">Email</label>
              <input
                id="e-email"
                type="email"
                value={form.email}
                onChange={(e) => setForm({ ...form, email: e.target.value })}
                disabled={Boolean(editing)}
                required
              />
              {editing ? <div className="tiny muted">Email tidak bisa diubah setelah karyawan dibuat.</div> : null}
            </div>
            <div className="field">
              <label htmlFor="e-role">Role</label>
              <select id="e-role" value={form.role} onChange={(e) => setForm({ ...form, role: e.target.value as Role })}>
                <option value="kasir">Kasir</option>
                <option value="owner">Owner</option>
              </select>
              <div className="tiny muted" style={{ marginTop: 6 }}>
                {roleDescriptions[form.role]}
              </div>
            </div>
            <div className="field">
              <label htmlFor="e-pin">{editing ? 'PIN baru (kosongkan bila tidak diganti)' : 'PIN login'}</label>
              <input
                id="e-pin"
                type="password"
                value={form.pin}
                onChange={(e) => setForm({ ...form, pin: e.target.value })}
                placeholder="4-12 karakter"
                required={!editing}
              />
            </div>
            <div className="row">
              <input
                id="e-active"
                type="checkbox"
                style={{ width: 'auto' }}
                checked={form.active}
                onChange={(e) => setForm({ ...form, active: e.target.checked })}
              />
              <label htmlFor="e-active" style={{ margin: 0 }}>
                Akun aktif (boleh masuk aplikasi)
              </label>
            </div>
          </form>
        </Modal>
      ) : null}
    </div>
  )
}
