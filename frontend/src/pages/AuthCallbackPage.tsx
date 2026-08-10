import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { useAuth } from '../context/AuthContext'

/**
 * AuthCallbackPage menerima token dari redirect Google. Token dikirim backend
 * lewat fragment URL (#token=...) agar tidak ikut tercatat di log server.
 */
export function AuthCallbackPage() {
  const { signIn } = useAuth()
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const params = new URLSearchParams(window.location.hash.replace(/^#/, ''))
    const token = params.get('token')
    if (!token) {
      setError('Token tidak ditemukan pada balasan Google.')
      return
    }
    // Bersihkan fragment agar token tidak tertinggal di riwayat peramban.
    window.history.replaceState(null, '', window.location.pathname)
    void signIn(token).then(
      () => navigate('/dashboard', { replace: true }),
      () => setError('Sesi gagal dibuat, silakan coba masuk lagi.'),
    )
  }, [signIn, navigate])

  return (
    <div className="auth-panel" style={{ minHeight: '100vh' }}>
      <div className="auth-box card card-pad">
        {error ? (
          <>
            <div className="alert alert-error">{error}</div>
            <button type="button" className="btn btn-block" style={{ marginTop: 14 }} onClick={() => navigate('/login')}>
              Kembali ke halaman masuk
            </button>
          </>
        ) : (
          <div className="row">
            <div className="spinner" style={{ borderTopColor: 'var(--brand)', borderColor: 'var(--border)' }} />
            <span>Menyiapkan akun dan folder Drive bisnis Anda...</span>
          </div>
        )}
      </div>
    </div>
  )
}
