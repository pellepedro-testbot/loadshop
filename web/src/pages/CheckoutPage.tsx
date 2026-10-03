import { useEffect, useState, type FormEvent } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { api, money, type Order, type Shipping } from '../api'
import { useAuth, useCart } from '../state'
import { ErrorBox, Spinner } from '../components/ui'
import { TotalsTable } from './CartPage'

const field = 'w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100'

export default function CheckoutPage() {
  const { username } = useAuth()
  const { cart, refresh, clearLocal } = useCart()
  const navigate = useNavigate()
  const [ship, setShip] = useState<Shipping>({
    name: '',
    email: username && username.includes('@') ? username : '',
    address: '',
    city: '',
    zip: '',
    country: 'United States',
  })
  const [card, setCard] = useState('4242 4242 4242 4242')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)

  useEffect(() => {
    refresh()
      .then(() => setLoadError(null))
      .catch((e: Error) => setLoadError(e.message))
  }, [refresh])

  const set = (k: keyof Shipping) => (e: React.ChangeEvent<HTMLInputElement>) => setShip({ ...ship, [k]: e.target.value })

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      const order = await api<Order>('/api/orders', {
        method: 'POST',
        body: JSON.stringify({ shipping: ship, payment: { cardNumber: card } }),
      })
      clearLocal()
      navigate(`/orders/${order.id}?placed=1`, { state: { order } })
    } catch (err) {
      setError((err as Error).message)
      setBusy(false)
    }
  }

  if (!cart) return loadError ? <ErrorBox message={loadError} /> : <Spinner label="Loading checkout" />
  if (cart.items.length === 0)
    return (
      <div className="rounded-2xl border border-dashed border-slate-300 bg-white py-16 text-center" data-testid="checkout-empty">
        <p className="text-slate-500">Your cart is empty.</p>
        <Link to="/" className="mt-4 inline-block text-sm font-medium text-indigo-600" data-testid="continue-shopping">Continue shopping</Link>
      </div>
    )

  return (
    <form onSubmit={onSubmit} className="grid gap-8 lg:grid-cols-3" data-testid="checkout-form">
      <div className="space-y-6 lg:col-span-2">
        <h1 className="text-2xl font-bold tracking-tight" data-testid="checkout-heading">Checkout</h1>
        {error && <ErrorBox message={error} />}
        <section className="rounded-2xl border border-slate-200 bg-white p-6">
          <h2 className="mb-4 text-lg font-semibold">Shipping address</h2>
          <div className="grid gap-4 sm:grid-cols-2">
            <label className="text-sm font-medium text-slate-700 sm:col-span-2">Full name
              <input required className={`${field} mt-1`} value={ship.name} onChange={set('name')} data-testid="checkout-name" autoComplete="name" />
            </label>
            <label className="text-sm font-medium text-slate-700 sm:col-span-2">Email
              <input required type="email" className={`${field} mt-1`} value={ship.email} onChange={set('email')} data-testid="checkout-email" autoComplete="email" />
            </label>
            <label className="text-sm font-medium text-slate-700 sm:col-span-2">Street address
              <input required className={`${field} mt-1`} value={ship.address} onChange={set('address')} data-testid="checkout-address" autoComplete="street-address" />
            </label>
            <label className="text-sm font-medium text-slate-700">City
              <input required className={`${field} mt-1`} value={ship.city} onChange={set('city')} data-testid="checkout-city" autoComplete="address-level2" />
            </label>
            <label className="text-sm font-medium text-slate-700">ZIP / Postal code
              <input required className={`${field} mt-1`} value={ship.zip} onChange={set('zip')} data-testid="checkout-zip" autoComplete="postal-code" />
            </label>
            <label className="text-sm font-medium text-slate-700 sm:col-span-2">Country
              <input required className={`${field} mt-1`} value={ship.country} onChange={set('country')} data-testid="checkout-country" autoComplete="country-name" />
            </label>
          </div>
        </section>
        <section className="rounded-2xl border border-slate-200 bg-white p-6">
          <h2 className="mb-1 text-lg font-semibold">Payment</h2>
          <p className="mb-4 text-sm text-slate-500">Demo store: no real payment is taken.</p>
          <label className="text-sm font-medium text-slate-700">Card number
            <input className={`${field} mt-1`} value={card} onChange={(e) => setCard(e.target.value)} data-testid="checkout-card" inputMode="numeric" autoComplete="cc-number" />
          </label>
        </section>
      </div>
      <aside className="h-fit rounded-2xl border border-slate-200 bg-white p-6 lg:mt-14">
        <h2 className="mb-4 text-lg font-semibold">Your order</h2>
        <ul className="mb-4 space-y-2 text-sm">
          {cart.items.map((l) => (
            <li key={l.productId} className="flex justify-between gap-3" data-testid={`checkout-line-${l.slug}`}>
              <span className="text-slate-700">{l.quantity} &times; {l.name}</span>
              <span>{money(l.lineTotal)}</span>
            </li>
          ))}
        </ul>
        <TotalsTable t={cart} prefix="checkout" />
        <button type="submit" disabled={busy} data-testid="place-order" className="mt-6 w-full rounded-xl bg-indigo-600 px-6 py-3 font-semibold text-white hover:bg-indigo-700 disabled:opacity-70">
          {busy ? 'Placing order...' : 'Place order'}
        </button>
      </aside>
    </form>
  )
}
