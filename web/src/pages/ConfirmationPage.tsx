import { Link, useLocation, useParams } from 'react-router-dom'
import { money, type Order } from '../api'
import { useApi } from '../hooks'
import { ErrorBox, Spinner } from '../components/ui'
import { TotalsTable } from './CartPage'

export default function ConfirmationPage() {
  const { id } = useParams()
  const loc = useLocation()
  const passed = (loc.state as { order?: Order } | null)?.order
  const fetched = useApi<Order>(passed?.id === id ? null : `/api/orders/${encodeURIComponent(id ?? '')}`)
  const order = passed?.id === id ? passed : fetched.data
  const justPlaced = new URLSearchParams(loc.search).has('placed')

  if (!order) return fetched.error ? <ErrorBox message={fetched.error} /> : <Spinner label="Loading order" />

  return (
    <div className="mx-auto max-w-2xl" data-testid="order-confirmation">
      <div className="rounded-3xl border border-slate-200 bg-white p-8 text-center shadow-sm">
        <div className="mx-auto mb-4 grid h-14 w-14 place-items-center rounded-full bg-emerald-100 text-emerald-600">
          <svg viewBox="0 0 24 24" className="h-7 w-7" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round"><path d="m5 12 5 5 9-10" /></svg>
        </div>
        <h1 className="text-2xl font-bold tracking-tight" data-testid="order-confirmation-heading">
          {justPlaced ? 'Thank you for your order!' : 'Order details'}
        </h1>
        <p className="mt-2 text-slate-500">
          Order <span className="font-mono font-semibold text-slate-900" data-testid="order-id">{order.id}</span> is{' '}
          <span className="font-medium text-emerald-700" data-testid="order-status">{order.status}</span>.
        </p>
        <p className="mt-1 text-sm text-slate-500">A confirmation was sent to {order.shipping.email}.</p>
      </div>

      <div className="mt-6 rounded-2xl border border-slate-200 bg-white p-6">
        <ul className="mb-4 divide-y divide-slate-100 text-sm">
          {order.items.map((l) => (
            <li key={l.productId} className="flex items-center justify-between gap-3 py-2" data-testid={`order-line-${l.slug}`}>
              <span className="flex items-center gap-3">
                <img src={l.image} alt="" className="h-10 w-14 rounded object-cover" />
                {l.quantity} &times; {l.name}
              </span>
              <span>{money(l.lineTotal)}</span>
            </li>
          ))}
        </ul>
        <TotalsTable t={order} prefix="order" />
        <div className="mt-4 border-t border-slate-200 pt-4 text-sm text-slate-600">
          <p className="font-medium text-slate-900">Shipping to</p>
          <p>{order.shipping.name}</p>
          <p>{order.shipping.address}, {order.shipping.city} {order.shipping.zip}</p>
          <p>Paid with card ending {order.paymentLast4}</p>
        </div>
      </div>

      <div className="mt-6 flex justify-center gap-3">
        <Link to="/" className="rounded-xl bg-indigo-600 px-5 py-2.5 text-sm font-semibold text-white hover:bg-indigo-700" data-testid="continue-shopping">Continue shopping</Link>
        <Link to="/orders" className="rounded-xl border border-slate-300 px-5 py-2.5 text-sm font-semibold text-slate-700 hover:bg-slate-50" data-testid="view-orders">View all orders</Link>
      </div>
    </div>
  )
}
