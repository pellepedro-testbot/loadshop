import { Link } from 'react-router-dom'
import { money, type Order } from '../api'
import { useApi } from '../hooks'
import { ErrorBox, Spinner } from '../components/ui'

export default function OrdersPage() {
  const { data, error } = useApi<{ items: Order[]; total: number }>('/api/orders')
  if (error) return <ErrorBox message={error} />
  if (!data) return <Spinner label="Loading orders" />
  return (
    <div>
      <h1 className="mb-6 text-2xl font-bold tracking-tight" data-testid="orders-heading">My orders</h1>
      {data.items.length === 0 ? (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-white py-16 text-center text-slate-500" data-testid="orders-empty">
          You have not placed any orders yet.
        </div>
      ) : (
        <div className="overflow-x-auto rounded-2xl border border-slate-200 bg-white">
          <table className="w-full text-sm" data-testid="orders-table">
            <thead className="bg-slate-50 text-left text-slate-600">
              <tr>
                <th className="px-4 py-3 font-medium">Order</th>
                <th className="px-4 py-3 font-medium">Date</th>
                <th className="px-4 py-3 font-medium">Items</th>
                <th className="px-4 py-3 font-medium">Status</th>
                <th className="px-4 py-3 text-right font-medium">Total</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {data.items.map((o) => (
                <tr key={o.id} data-testid={`order-row-${o.id}`}>
                  <td className="px-4 py-3">
                    <Link to={`/orders/${o.id}`} className="font-mono font-semibold text-indigo-600 hover:text-indigo-800" data-testid={`order-link-${o.id}`}>{o.id}</Link>
                  </td>
                  <td className="px-4 py-3 text-slate-600">{new Date(o.createdAt).toLocaleString()}</td>
                  <td className="px-4 py-3 text-slate-600">{o.itemCount}</td>
                  <td className="px-4 py-3"><span className="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700">{o.status}</span></td>
                  <td className="px-4 py-3 text-right font-semibold">{money(o.total)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
