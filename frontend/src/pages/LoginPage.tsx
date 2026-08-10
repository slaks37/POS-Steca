import { useState } from 'react'
import { Navigate, useSearchParams } from 'react-router-dom'

import { Brand, ErrorAlert } from '../components/ui'
import { useAuth } from '../context/AuthContext'
import { ApiError, isNative, request } from '../lib/api'
import type { User } from '../lib/types'

interface StaffLoginResponse {
  token: string
  user: User
}

interface DemoLoginResponse {
  token: string
  kasir_demo: { email: string; pin: string }
  tenant: { code: string }
}

const features = [
  {
    icon: '🗂️',
    title: 'Data tersimpan di Google Drive Anda',
    body: 'Setiap bisnis punya satu folder Drive dan satu spreadsheet sendiri. Datanya tetap milik Anda.',
  },
  {
    icon: '🧾',
    title: 'Kasir cepat dengan struk digital',
    body: 'Pilih produk, hitung kembalian, cetak struk. Tunai, QRIS, dan kartu siap dipakai.',
  },
  {
    icon: '🍽️',
    title: 'Pesanan F&B terpantau',
    body: 'Papan pesanan baru, diproses, dan selesai — dari kasir langsung maupun pemesanan online.',
  },
]

/** LoginPage menyediakan dua jalur masuk: pemilik lewat Google, kasir lewat PIN. */
export function LoginPage() {
  const { user, loading, signIn } = useAuth()
  const [searchParams] = useSearchParams()
  const [tenantCode, setTenantCode] = useState('')
  const [email, setEmail] = useState('')
  const [pin, setPin] = useState('')
  const [error, setError] = useState<string | null>(searchParams.get('error'))
  const [busy, setBusy] = useState(false)
  const [demoHint, setDemoHint] = useState<string | null>(null)

  if (!loading && user) {
    return <Navigate to={user.role === 'owner' ? '/dashboard' : '/kasir'} replace />
  }

  async function handleGoogleLogin() {
    setBusy(true)
    setError(null)
    try {
      const res = await request<{ url: string }>('/auth/google/url', { auth: false })
      window.location.href = res.url
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal menghubungi server')
      setBusy(false)
    }
  }

  async function handleStaffLogin(e: React.FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const res = await request<StaffLoginResponse>('/auth/staff/login', {
        method: 'POST',
        auth: false,
        body: { tenant_code: tenantCode, email, pin },
      })
      await signIn(res.token)
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Gagal masuk')
    } finally {
      setBusy(false)
    }
  }

  async function handleDemoLogin() {
    setBusy(true)
    setError(null)
    try {
      const res = await request<DemoLoginResponse>('/auth/demo/login', { method: 'POST', auth: false, body: {} })
      setDemoHint(`Kode bisnis demo: ${res.tenant.code} — kasir: ${res.kasir_demo.email} / PIN ${res.kasir_demo.pin}`)
      await signIn(res.token)
    } catch (err) {
      setError(
        err instanceof ApiError && err.status === 404
          ? 'Mode demo tidak aktif. Jalankan backend dengan POS_DATASTORE=memory untuk mencobanya.'
          : 'Gagal masuk ke mode demo',
      )
      setBusy(false)
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-hero">
        <Brand size="lg" tagline="Point of Sale UMKM" />
        <h2>Kasir modern untuk UMKM Indonesia</h2>
        <p style={{ maxWidth: 460, margin: 0 }}>
          Kelola penjualan, stok, dan pesanan dari satu tempat — tanpa server database. Semua data tercatat rapi di
          Google Sheets milik bisnis Anda sendiri.
        </p>
        <div className="stack" style={{ gap: 14, marginTop: 8 }}>
          {features.map((f) => (
            <div className="auth-feature" key={f.title}>
              <span className="auth-feature-icon" aria-hidden>
                {f.icon}
              </span>
              <div>
                <strong>{f.title}</strong>
                <div style={{ fontSize: 14, opacity: 0.9 }}>{f.body}</div>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="auth-panel">
        <div className="auth-box">
          <h2 style={{ marginBottom: 6 }}>Masuk</h2>
          <p className="muted small" style={{ marginTop: 0 }}>
            Pemilik masuk dengan akun Google bisnis. Karyawan masuk dengan kode bisnis dan PIN.
          </p>

          <div className="stack" style={{ gap: 12 }}>
            <ErrorAlert message={error} />
            {demoHint ? <div className="alert alert-info">{demoHint}</div> : null}

            {isNative ? (
              // Google memblokir alur OAuth di dalam WebView aplikasi
              // (galat "disallowed_useragent"), jadi onboarding pemilik
              // dilakukan sekali lewat peramban.
              <div className="alert alert-info small">
                <span aria-hidden>ℹ️</span>
                <span>
                  Hubungkan akun Google sekali lewat peramban di{' '}
                  <strong>versi web Steca POS</strong>, lalu atur PIN Anda di menu Karyawan. Setelah itu masuk di
                  aplikasi ini memakai kode bisnis, email, dan PIN tersebut.
                </span>
              </div>
            ) : (
              <>
                <button type="button" className="google-btn" onClick={handleGoogleLogin} disabled={busy}>
                  <span aria-hidden>🔐</span>
                  Masuk sebagai Pemilik dengan Google
                </button>
                <div className="tiny muted">
                  Aplikasi meminta izin membuat folder dan spreadsheet di Drive Anda. Izin dibatasi pada berkas yang
                  dibuat aplikasi ini saja.
                </div>
              </>
            )}
          </div>

          <div className="divider">{isNative ? 'masuk ke aplikasi' : 'atau masuk sebagai karyawan'}</div>

          <form onSubmit={handleStaffLogin}>
            <div className="field">
              <label htmlFor="tenant-code">Kode bisnis</label>
              <input
                id="tenant-code"
                value={tenantCode}
                onChange={(e) => setTenantCode(e.target.value.toUpperCase())}
                placeholder="STC-XXXXX"
                autoComplete="organization"
                required
              />
            </div>
            <div className="field">
              <label htmlFor="email">Email karyawan</label>
              <input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="kasir@tokoanda.com"
                autoComplete="username"
                required
              />
            </div>
            <div className="field">
              <label htmlFor="pin">PIN</label>
              <input
                id="pin"
                type="password"
                value={pin}
                onChange={(e) => setPin(e.target.value)}
                placeholder="••••••"
                autoComplete="current-password"
                required
              />
            </div>
            <button type="submit" className="btn btn-block" disabled={busy}>
              {busy ? 'Memproses...' : 'Masuk'}
            </button>
          </form>

          {isNative ? null : (
            <>
              <div className="divider">coba tanpa akun</div>
              <button type="button" className="btn btn-secondary btn-block" onClick={handleDemoLogin} disabled={busy}>
                Masuk mode demo
              </button>
              <div className="tiny muted" style={{ marginTop: 8 }}>
                Mode demo memakai data contoh di memori server dan hanya tersedia saat backend dijalankan dengan
                POS_DATASTORE=memory.
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
