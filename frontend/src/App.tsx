import type { ReactElement } from 'react'
import { BrowserRouter, HashRouter, Navigate, Route, Routes } from 'react-router-dom'

import { Layout } from './components/Layout'
import { AuthProvider, useAuth } from './context/AuthContext'
import { AuthCallbackPage } from './pages/AuthCallbackPage'
import { CashierPage } from './pages/CashierPage'
import { CustomersPage } from './pages/CustomersPage'
import { DashboardPage } from './pages/DashboardPage'
import { EmployeesPage } from './pages/EmployeesPage'
import { LoginPage } from './pages/LoginPage'
import { OrdersPage } from './pages/OrdersPage'
import { ProductsPage } from './pages/ProductsPage'
import { PublicMenuPage } from './pages/PublicMenuPage'
import { ReportsPage } from './pages/ReportsPage'
import { TablesPage } from './pages/TablesPage'
import { SettingsPage } from './pages/SettingsPage'
import { isNative } from './lib/api'

/**
 * Di web dipakai BrowserRouter agar URL tetap bersih. Di WebView Android
 * berkas dilayani dari bundel aset, jadi HashRouter dipilih supaya navigasi
 * langsung ke rute dalam (mis. /kasir) tidak bergantung pada fallback server.
 */
const Router = isNative ? HashRouter : BrowserRouter

function FullScreenLoader() {
  return (
    <div className="auth-panel" style={{ minHeight: '100vh' }}>
      <div className="row">
        <div className="spinner" style={{ borderTopColor: 'var(--brand)', borderColor: 'var(--border)' }} />
        <span className="muted">Memuat sesi...</span>
      </div>
    </div>
  )
}

/** RequireAuth melindungi rute aplikasi; ownerOnly membatasi ke pemilik. */
function RequireAuth({ children, ownerOnly = false }: { children: ReactElement; ownerOnly?: boolean }) {
  const { user, loading, isOwner } = useAuth()
  if (loading) return <FullScreenLoader />
  if (!user) return <Navigate to="/login" replace />
  if (ownerOnly && !isOwner) return <Navigate to="/kasir" replace />
  return children
}

/** HomeRedirect mengarahkan pengguna ke halaman awal sesuai rolenya. */
function HomeRedirect() {
  const { user, loading, isOwner } = useAuth()
  if (loading) return <FullScreenLoader />
  if (!user) return <Navigate to="/login" replace />
  return <Navigate to={isOwner ? '/dashboard' : '/kasir'} replace />
}

export default function App() {
  return (
    <Router>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/auth/callback" element={<AuthCallbackPage />} />
          <Route path="/menu/:code" element={<PublicMenuPage />} />

          <Route
            element={
              <RequireAuth>
                <Layout />
              </RequireAuth>
            }
          >
            <Route
              path="/dashboard"
              element={
                <RequireAuth ownerOnly>
                  <DashboardPage />
                </RequireAuth>
              }
            />
            <Route path="/kasir" element={<CashierPage />} />
            <Route path="/pesanan" element={<OrdersPage />} />
            <Route path="/meja" element={<TablesPage />} />
            <Route path="/produk" element={<ProductsPage />} />
            <Route
              path="/pelanggan"
              element={
                <RequireAuth ownerOnly>
                  <CustomersPage />
                </RequireAuth>
              }
            />
            <Route
              path="/laporan"
              element={
                <RequireAuth ownerOnly>
                  <ReportsPage />
                </RequireAuth>
              }
            />
            <Route
              path="/karyawan"
              element={
                <RequireAuth ownerOnly>
                  <EmployeesPage />
                </RequireAuth>
              }
            />
            <Route path="/pengaturan" element={<SettingsPage />} />
          </Route>

          <Route path="/" element={<HomeRedirect />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </AuthProvider>
    </Router>
  )
}
