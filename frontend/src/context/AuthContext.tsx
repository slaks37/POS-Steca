import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'

import { clearToken, getToken, request, setToken } from '../lib/api'
import type { Tenant, User } from '../lib/types'

interface MeResponse {
  user: User
  tenant: Tenant
}

interface AuthState {
  user: User | null
  tenant: Tenant | null
  loading: boolean
  isOwner: boolean
  signIn: (token: string) => Promise<void>
  signOut: () => void
  refresh: () => Promise<void>
  setTenant: (tenant: Tenant) => void
}

const AuthContext = createContext<AuthState | undefined>(undefined)

/** AuthProvider menyimpan sesi aktif dan memuat profil tenant dari backend. */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [tenant, setTenantState] = useState<Tenant | null>(null)
  const [loading, setLoading] = useState(true)

  const load = useCallback(async () => {
    if (!getToken()) {
      setUser(null)
      setTenantState(null)
      setLoading(false)
      return
    }
    try {
      const me = await request<MeResponse>('/auth/me')
      setUser(me.user)
      setTenantState(me.tenant)
    } catch {
      clearToken()
      setUser(null)
      setTenantState(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const signIn = useCallback(
    async (token: string) => {
      setToken(token)
      setLoading(true)
      await load()
    },
    [load],
  )

  const signOut = useCallback(() => {
    clearToken()
    setUser(null)
    setTenantState(null)
  }, [])

  const value = useMemo<AuthState>(
    () => ({
      user,
      tenant,
      loading,
      isOwner: user?.role === 'owner',
      signIn,
      signOut,
      refresh: load,
      setTenant: setTenantState,
    }),
    [user, tenant, loading, signIn, signOut, load],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

/** useAuth mengambil state sesi; harus dipakai di dalam AuthProvider. */
export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth harus dipakai di dalam AuthProvider')
  return ctx
}
