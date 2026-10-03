export const TOKEN_KEY = 'loadshop_token'
export const USER_KEY = 'loadshop_user'

export interface Spec { key: string; value: string }

export interface Product {
  id: number
  slug: string
  name: string
  brand: string
  category: string
  categorySlug: string
  price: number
  rating: number
  reviewCount: number
  shortDescription: string
  description?: string
  specs?: Spec[]
  stock: number
  image: string
}

export interface Category { slug: string; name: string; count: number }

export interface CartLine {
  productId: number
  slug: string
  name: string
  brand: string
  image: string
  price: number
  quantity: number
  lineTotal: number
}

export interface Totals {
  itemCount: number
  subtotal: number
  tax: number
  shippingCost: number
  total: number
}

export interface Cart extends Totals { items: CartLine[] }

export interface Shipping {
  name: string
  email: string
  address: string
  city: string
  zip: string
  country: string
}

export interface Order extends Totals {
  id: string
  username: string
  status: string
  createdAt: string
  items: CartLine[]
  shipping: Shipping
  paymentLast4: string
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

// Read on every call so tokens injected into localStorage by a test harness are honoured.
export function getToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY)
  } catch {
    return null
  }
}

let onUnauthorized: (() => void) | null = null
export function setUnauthorizedHandler(fn: (() => void) | null) {
  onUnauthorized = fn
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
  const token = getToken()
  if (token) headers.set('Authorization', `Bearer ${token}`)
  const res = await fetch(path, { ...init, headers })
  if (!res.ok) {
    let msg = res.statusText
    try {
      msg = (await res.json()).error ?? msg
    } catch {
      /* not JSON */
    }
    if (res.status === 401 && token && onUnauthorized) onUnauthorized()
    throw new ApiError(res.status, msg)
  }
  return res.json() as Promise<T>
}

export const money = (n: number) =>
  n.toLocaleString('en-US', { style: 'currency', currency: 'USD' })
