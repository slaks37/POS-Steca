import { useState } from 'react'

import { ErrorAlert } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { ApiError, request } from '../lib/api'
import { formatDateTime } from '../lib/format'
import type { Tenant } from '../lib/types'

/** SettingsPage menampilkan profil bisnis, penyimpanan data, dan kanal online. */
export function SettingsPage() {
  const { tenant, isOwner, setTenant } = useAuth()
  const [businessName, setBusinessName] = useState(tenant?.business_name ?? '')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)

  const menuLink = tenant ? `${window.location.origin}/menu/${tenant.code}` : ''

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    setError(null)
    setSaved(false)
    try {
      const res = await request<{ tenant: Tenant }>('/tenant', { method: 'PATCH', body: { business_name: businessName } })
      setTenant(res.tenant)
      setSaved(true)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal menyimpan perubahan')
    } finally {
      setSaving(false)
    }
  }

  if (!tenant) return null

  return (
    <div className="stack">
      <div className="grid grid-2">
        <div className="card">
          <div className="card-header">
            <h3>Profil bisnis</h3>
          </div>
          <div className="card-pad">
            <ErrorAlert message={error} />
            {saved ? <div className="alert alert-success">Perubahan tersimpan.</div> : null}
            <form onSubmit={handleSave} style={{ marginTop: 12 }}>
              <div className="field">
                <label htmlFor="business-name">Nama bisnis</label>
                <input
                  id="business-name"
                  value={businessName}
                  onChange={(e) => setBusinessName(e.target.value)}
                  disabled={!isOwner}
                  required
                />
              </div>
              <div className="field">
                <label>Kode bisnis (untuk login karyawan)</label>
                <input value={tenant.code} readOnly />
              </div>
              <div className="field">
                <label>Akun Google pemilik</label>
                <input value={tenant.owner_email} readOnly />
              </div>
              <div className="tiny muted" style={{ marginBottom: 12 }}>
                Terhubung sejak {formatDateTime(tenant.created_at)}
              </div>
              {isOwner ? (
                <button type="submit" className="btn" disabled={saving}>
                  {saving ? 'Menyimpan...' : 'Simpan perubahan'}
                </button>
              ) : (
                <div className="alert alert-info small">Hanya pemilik yang bisa mengubah profil bisnis.</div>
              )}
            </form>
          </div>
        </div>

        <div className="stack">
          <div className="card">
            <div className="card-header">
              <h3>Penyimpanan data</h3>
            </div>
            <div className="card-pad stack" style={{ gap: 12 }}>
              <p className="small muted" style={{ margin: 0 }}>
                Produk, transaksi, pesanan, pelanggan, dan karyawan tersimpan di database PostgreSQL aplikasi, terpisah
                per bisnis. Gambar produk disimpan di folder Google Drive milik akun Anda sendiri.
              </p>
              {tenant.folder_url ? (
                <a className="btn btn-secondary" href={tenant.folder_url} target="_blank" rel="noreferrer">
                  🗂️ Buka folder gambar produk
                </a>
              ) : (
                <div className="alert alert-info small">
                  Folder Drive belum tersedia pada mode datastore ini.
                </div>
              )}
            </div>
          </div>

          <div className="card">
            <div className="card-header">
              <h3>Kanal pemesanan online</h3>
            </div>
            <div className="card-pad stack" style={{ gap: 10 }}>
              <p className="small muted" style={{ margin: 0 }}>
                Bagikan tautan berikut ke pelanggan. Pesanan yang masuk otomatis muncul di papan pesanan dengan sumber
                &quot;online&quot;.
              </p>
              <div className="row" style={{ flexWrap: 'nowrap' }}>
                <input value={menuLink} readOnly />
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => void navigator.clipboard?.writeText(menuLink)}
                >
                  Salin
                </button>
              </div>
              <a className="btn btn-secondary" href={`/menu/${tenant.code}`} target="_blank" rel="noreferrer">
                Pratinjau halaman menu
              </a>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
