import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { api, getToken, setUnauthorizedHandler, TOKEN_KEY, USER_KEY, type Cart } from './api'

interface AuthState {
  token: string | null
  username: string | null
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

interface CartState {
  cart: Cart | null
  count: number
  refresh: () => Promise<void>
  add: (productId: number, quantity?: number) => Promise<void>
  setQuantity: (productId: number, quantity: number) => Promise<void>
  remove: (productId: number) => Promise<void>
  clearLocal: () => void
}

const AuthCtx = createContext<AuthState | null>(null)
const CartCtx = createContext<CartState | null>(null)

function readUser(): string | null {
  try {
    return localStorage.getItem(USER_KEY)
  } catch {
    return null
  }
}

export function AppProviders({ children }: { children: ReactNode }) {
  // Initial state comes straight from localStorage, so a pre-injected token works without a login step.
  const [token, setToken] = useState<string | null>(getToken)
  const [username, setUsername] = useState<string | null>(readUser)
  const [cart, setCart] = useState<Cart | null>(null)

  const logout = useCallback(() => {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
    setToken(null)
    setUsername(null)
    setCart(null)
  }, [])

  useEffect(() => {
    setUnauthorizedHandler(logout)
    return () => setUnauthorizedHandler(null)
  }, [logout])

  // Pick up tokens written to localStorage by other tabs.
  useEffect(() => {
    const onStorage = (e: StorageEvent) => {
      if (e.key === TOKEN_KEY || e.key === USER_KEY) {
        setToken(getToken())
        setUsername(readUser())
      }
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  // Token injected without a username: ask the server who we are.
  useEffect(() => {
    if (token && !username) {
      api<{ username: string }>('/api/auth/me')
        .then((r) => {
          localStorage.setItem(USER_KEY, r.username)
          setUsername(r.username)
        })
        .catch(() => {})
    }
  }, [token, username])

  const login = useCallback(async (u: string, p: string) => {
    const r = await api<{ access_token: string; username: string }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username: u, password: p }),
    })
    localStorage.setItem(TOKEN_KEY, r.access_token)
    localStorage.setItem(USER_KEY, r.username)
    setToken(r.access_token)
    setUsername(r.username)
  }, [])

  const refresh = useCallback(async () => {
    if (!getToken()) {
      setCart(null)
      return
    }
    setCart(await api<Cart>('/api/cart'))
  }, [])

  useEffect(() => {
    refresh().catch(() => {})
  }, [token, refresh])

  const add = useCallback(async (productId: number, quantity = 1) => {
    setCart(await api<Cart>('/api/cart/items', { method: 'POST', body: JSON.stringify({ productId, quantity }) }))
  }, [])
  const setQuantity = useCallback(async (productId: number, quantity: number) => {
    setCart(await api<Cart>(`/api/cart/items/${productId}`, { method: 'PATCH', body: JSON.stringify({ quantity }) }))
  }, [])
  const remove = useCallback(async (productId: number) => {
    setCart(await api<Cart>(`/api/cart/items/${productId}`, { method: 'DELETE' }))
  }, [])
  const clearLocal = useCallback(() => setCart((c) => (c ? { ...c, items: [], itemCount: 0, subtotal: 0, tax: 0, shippingCost: 0, total: 0 } : c)), [])

  const auth = useMemo(() => ({ token, username, login, logout }), [token, username, login, logout])
  const cartState = useMemo(
    () => ({ cart, count: cart?.itemCount ?? 0, refresh, add, setQuantity, remove, clearLocal }),
    [cart, refresh, add, setQuantity, remove, clearLocal],
  )

  return (
    <AuthCtx.Provider value={auth}>
      <CartCtx.Provider value={cartState}>{children}</CartCtx.Provider>
    </AuthCtx.Provider>
  )
}

export function useAuth() {
  const v = useContext(AuthCtx)
  if (!v) throw new Error('useAuth outside provider')
  return v
}

export function useCart() {
  const v = useContext(CartCtx)
  if (!v) throw new Error('useCart outside provider')
  return v
}
