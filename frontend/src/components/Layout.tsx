import { NavLink, Outlet, useLocation } from 'react-router-dom'

import { useAuth } from '../context/AuthContext'
import { initials } from '../lib/format'
import { Brand } from './ui'

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
  { to: '/meja', label: 'Meja', icon: '🪑' },
  { to: '/produk', label: 'Produk', icon: '📦' },
  { to: '/pelanggan', label: 'Pelanggan', icon: '💳', ownerOnly: true },
  { to: '/laporan', label: 'Laporan', icon: '📈', ownerOnly: true },
  { to: '/karyawan', label: 'Karyawan', icon: '👥', ownerOnly: true },
  { to: '/pengaturan', label: 'Pengaturan', icon: '⚙️' },
]

const pages: Record<string, { title: string; sub: string }> = {
  '/dashboard': { title: 'Dashboard', sub: 'Ringkasan performa toko hari ini' },
  '/kasir': { title: 'Kasir', sub: 'Catat transaksi dan cetak struk' },
  '/pesanan': { title: 'Manajemen Pesanan', sub: 'Pantau pesanan dari kasir dan kanal online' },
  '/meja': { title: 'Denah Meja', sub: 'Status meja dan kapasitasnya' },
  '/produk': { title: 'Produk & Stok', sub: 'Katalog yang tersinkron dengan Google Sheets' },
  '/pelanggan': { title: 'Pelanggan & Loyalitas', sub: 'Riwayat belanja dan poin pelanggan' },
  '/laporan': { title: 'Laporan Penjualan', sub: 'Omzet, produk terlaris, dan performa kasir' },
  '/karyawan': { title: 'Karyawan', sub: 'Akun dan level akses tim Anda' },
  '/pengaturan': { title: 'Pengaturan', sub: 'Profil bisnis dan penyimpanan data' },
}

/** Layout adalah kerangka aplikasi: sidebar navigasi + topbar + konten. */
export function Layout() {
  const { user, tenant, isOwner, signOut } = useAuth()
  const location = useLocation()
  const page = pages[location.pathname] ?? { title: 'Steca POS', sub: '' }

  const visibleItems = navItems.filter((item) => !item.ownerOnly || isOwner)

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <Brand tagline="Kasir UMKM" />
        <div className="sidebar-section">Menu</div>
        {visibleItems.map((item) => (
          <NavLink key={item.to} to={item.to} className={({ isActive }) => `nav-link${isActive ? ' active' : ''}`}>
            <span className="nav-icon" aria-hidden>
              {item.icon}
            </span>
            {item.label}
          </NavLink>
        ))}
        <div className="sidebar-footer">
          <div className="biz">{tenant?.business_name}</div>
          <div className="code">{tenant?.code}</div>
        </div>
      </aside>

      <div className="main">
        <header className="topbar">
          <div>
            <h1>{page.title}</h1>
            <div className="topbar-sub">{page.sub || tenant?.business_name}</div>
          </div>
          <div className="user-chip">
            <div className="avatar">{initials(user?.name ?? '?')}</div>
            <div className="user-meta">
              <strong>{user?.name}</strong>
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
