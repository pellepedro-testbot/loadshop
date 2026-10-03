import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { money, type Totals } from '../api'
import { useCart } from '../state'
import { ErrorBox, Spinner } from '../components/ui'

export function TotalsTable({ t, prefix }: { t: Totals; prefix: string }) {
  return (
    <dl className="space-y-2 text-sm">
      <div className="flex justify-between"><dt className="text-slate-600">Subtotal</dt><dd data-testid={`${prefix}-subtotal`}>{money(t.subtotal)}</dd></div>
      <div className="flex justify-between"><dt className="text-slate-600">Tax (8%)</dt><dd data-testid={`${prefix}-tax`}>{money(t.tax)}</dd></div>
      <div className="flex justify-between"><dt className="text-slate-600">Shipping</dt><dd data-testid={`${prefix}-shipping`}>{t.shippingCost === 0 ? "Free" : money(t.shippingCost)}</dd></div>
      <div className="flex justify-between border-t border-slate-200 pt-3 text-base font-semibold"><dt>Total</dt><dd data-testid={`${prefix}-total`}>{money(t.total)}</dd></div>
    </dl>
  )
}

export default function CartPage() {
  const { cart, refresh, setQuantity, remove } = useCart()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    refresh().catch((e: Error) => setError(e.message))
  }, [refresh])

  const run = (p: Promise<void>) => p.then(() => setError(null)).catch((e: Error) => setError(e.message))

  if (!cart) return error ? <ErrorBox message={error} /> : <Spinner label="Loading cart" />

  return (
    <div>
      <h1 className="mb-6 text-2xl font-bold tracking-tight" data-testid="cart-heading">Shopping cart</h1>
      {error && <div className="mb-4"><ErrorBox message={error} /></div>}
      {cart.items.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-white py-16 text-center" data-testid="cart-empty">
          <p className="text-slate-500">Your cart is empty.</p>
          <Link to="/" className="mt-4 inline-block rounded-lg bg-indigo-600 px-5 py-2 text-sm font-medium text-white hover:bg-indigo-700" data-testid="continue-shopping">
            Continue shopping
          </Link>
        </div>
      ) : (
        <div className="grid gap-8 lg:grid-cols-3">
          <ul className="divide-y divide-slate-200 overflow-hidden rounded-2xl border border-slate-200 bg-white lg:col-span-2">
            {cart.items.map((l) => (
              <li key={l.productId} className="flex gap-4 p-4" data-testid={`cart-line-${l.slug}`}>
                <Link to={`/product/${l.slug}`} className="shrink-0">
                  <img src={l.image} alt="" className="h-24 w-32 rounded-lg object-cover" />
                </Link>
                <div className="flex flex-1 flex-col">
                  <div className="flex justify-between gap-4">
                    <div>
                      <p className="text-xs font-medium uppercase text-indigo-600">{l.brand}</p>
                      <Link to={`/product/${l.slug}`} className="font-semibold hover:text-indigo-700" data-testid={`cart-line-name-${l.slug}`}>{l.name}</Link>
                      <p className="text-sm text-slate-500">{money(l.price)} each</p>
                    </div>
                    <p className="font-semibold" data-testid={`cart-line-total-${l.slug}`}>{money(l.lineTotal)}</p>
                  </div>
                  <div className="mt-auto flex items-center gap-3 pt-2">
                    <label className="flex items-center gap-2 text-sm text-slate-600">
                      Qty
                      <select
                        value={l.quantity}
                        data-testid={`cart-qty-${l.slug}`}
                        onChange={(e) => run(setQuantity(l.productId, Number(e.target.value)))}
                        className="rounded-lg border border-slate-300 bg-white px-2 py-1 text-sm"
                      >
                        {Array.from({ length: Math.max(10, l.quantity) }, (_, i) => i + 1).map((n) => (
                          <option key={n} value={n}>{n}</option>
                        ))}
                      </select>
                    </label>
                    <button type="button" onClick={() => run(remove(l.productId))} data-testid={`cart-remove-${l.slug}`} className="text-sm font-medium text-rose-600 hover:text-rose-800">
                      Remove
                    </button>
                  </div>
                </div>
              </li>
            ))}
          </ul>
          <aside className="h-fit rounded-2xl border border-slate-200 bg-white p-6">
            <h2 className="mb-4 text-lg font-semibold">Order summary</h2>
            <TotalsTable t={cart} prefix="cart" />
            <Link to="/checkout" data-testid="checkout-button" className="mt-6 block rounded-xl bg-indigo-600 px-6 py-3 text-center font-semibold text-white hover:bg-indigo-700">
              Proceed to checkout
            </Link>
            <Link to="/" className="mt-3 block text-center text-sm text-slate-500 hover:text-slate-800" data-testid="continue-shopping">
              Continue shopping
            </Link>
          </aside>
        </div>
      )}
    </div>
  )
}
