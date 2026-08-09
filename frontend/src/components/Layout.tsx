import { NavLink, Outlet, useLocation } from 'react-router-dom'

import { useAuth } from '../context/AuthContext'
import { initials } from '../lib/format'

interface NavItem {
  to: string
  label: string
  icon: string
  ownerOnly?: boolean
}

const navItems: NavItem[] = [
  { to: '/dashboard', label: 'Dashboard', icon: '📊', ownerOnly: true },
  { to: '/kasir', label: 'Kasir', icon: '🧾' },
  { to: '/pesanan', label: 'Pesanan', icon: '🍽️' },
  { to: '/produk', label: 'Produk & Stok', icon: '📦' },
  { to: '/laporan', label: 'Laporan', icon: '📈', ownerOnly: true },
  { to: '/karyawan', label: 'Karyawan', icon: '👥', ownerOnly: true },
  { to: '/pengaturan', label: 'Pengaturan', icon: '⚙️' },
]

const pageTitles: Record<string, string> = {
  '/dashboard': 'Dashboard',
  '/kasir': 'Kasir',
  '/pesanan': 'Manajemen Pesanan',
  '/produk': 'Produk & Stok',
  '/laporan': 'Laporan Penjualan',
  '/karyawan': 'Karyawan',
  '/pengaturan': 'Pengaturan',
}

/** Layout adalah kerangka aplikasi: sidebar navigasi + topbar + konten. */
export function Layout() {
  const { user, tenant, isOwner, signOut } = useAuth()
  const location = useLocation()
  const title = pageTitles[location.pathname] ?? 'POS Steca'

  const visibleItems = navItems.filter((item) => !item.ownerOnly || isOwner)

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <span className="sidebar-logo">S</span>
          <span>POS Steca</span>
        </div>
        <div className="sidebar-section">Menu</div>
        {visibleItems.map((item) => (
          <NavLink key={item.to} to={item.to} className={({ isActive }) => `nav-link${isActive ? ' active' : ''}`}>
            <span aria-hidden>{item.icon}</span>
            {item.label}
          </NavLink>
        ))}
        <div className="sidebar-footer">
          <div style={{ color: '#fff', fontWeight: 600 }}>{tenant?.business_name}</div>
          <div className="tiny" style={{ color: '#5eead4' }}>
            Kode bisnis: {tenant?.code}
          </div>
        </div>
      </aside>

      <div className="main">
        <header className="topbar">
          <div>
            <h1>{title}</h1>
            <div className="topbar-sub">{tenant?.business_name ?? 'Memuat...'}</div>
          </div>
          <div className="user-chip">
            <div className="avatar">{initials(user?.name ?? '?')}</div>
            <div>
              <div style={{ fontWeight: 600, fontSize: 14 }}>{user?.name}</div>
              <div className="tiny muted" style={{ textTransform: 'capitalize' }}>
                {user?.role}
              </div>
            </div>
            <button type="button" className="btn btn-secondary btn-sm" onClick={signOut}>
              Keluar
            </button>
          </div>
        </header>

        <main className="page">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
