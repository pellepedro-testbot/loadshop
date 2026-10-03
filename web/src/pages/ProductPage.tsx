import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { money, type Product } from '../api'
import { useApi } from '../hooks'
import { ErrorBox, Spinner, Stars, useAddToCart } from '../components/ui'

export default function ProductPage() {
  const { slug } = useParams()
  const { data: p, error, loading } = useApi<Product>(`/api/products/${encodeURIComponent(slug ?? '')}`)
  const addToCart = useAddToCart()
  const [qty, setQty] = useState(1)
  const [status, setStatus] = useState<'idle' | 'busy' | 'added' | 'error'>('idle')
  const [msg, setMsg] = useState('')

  // The page is reused across /product/:slug, so reset per-product state.
  useEffect(() => {
    setQty(1)
    setStatus('idle')
    setMsg('')
  }, [slug])

  const onAdd = async () => {
    if (!p) return
    setStatus('busy')
    try {
      const ok = await addToCart(p.id, qty)
      setStatus(ok ? 'added' : 'idle')
    } catch (e) {
      setStatus('error')
      setMsg((e as Error).message)
    }
  }

  const back = (
    <Link to="/" data-testid="back-to-catalog" className="inline-flex items-center gap-1 text-sm font-medium text-indigo-600 hover:text-indigo-800">
      <svg viewBox="0 0 20 20" className="h-4 w-4" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 15l-5-5 5-5" /></svg>
      Back to catalog
    </Link>
  )

  if (loading && !p) return <>{back}<Spinner label="Loading product" /></>
  if (error || !p) return <div className="space-y-4">{back}<ErrorBox message={error ?? 'Product not found'} /></div>

  return (
    <div className="space-y-6" data-testid="product-detail">
      <nav className="flex items-center gap-3 text-sm text-slate-500">
        {back}
        <span aria-hidden="true">/</span>
        <Link to={`/category/${p.categorySlug}`} className="hover:text-slate-800" data-testid="product-detail-category">{p.category}</Link>
      </nav>

      <div className="grid gap-8 lg:grid-cols-2">
        <div className="self-start overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-sm">
          <img src={p.image} alt={p.name} width={600} height={450} className="aspect-[4/3] w-full object-cover" data-testid="product-detail-image" />
        </div>

        <div className="flex flex-col gap-4">
          <span className="text-sm font-semibold uppercase tracking-wide text-indigo-600" data-testid="product-detail-brand">{p.brand}</span>
          <h1 className="text-3xl font-bold tracking-tight text-slate-900 sm:text-4xl" data-testid="product-detail-name">{p.name}</h1>
          <Stars rating={p.rating} count={p.reviewCount} testId="product-detail-rating" />
          <p className="text-3xl font-bold text-slate-900" data-testid="product-detail-price">{money(p.price)}</p>
          <p className="text-slate-600" data-testid="product-detail-description">{p.description}</p>
          <p className={`text-sm font-medium ${p.stock > 20 ? 'text-emerald-600' : 'text-amber-600'}`} data-testid="product-detail-stock">
            {p.stock > 20 ? 'In stock' : `Only ${p.stock} left`} &middot; ships in 1-2 business days
          </p>

          <div className="mt-2 flex flex-wrap items-center gap-3">
            <div className="flex items-center rounded-xl border border-slate-300 bg-white">
              <button type="button" aria-label="Decrease quantity" data-testid="quantity-decrease" onClick={() => setQty((q) => Math.max(1, q - 1))} className="px-3 py-2 text-lg text-slate-600 hover:text-slate-900">&minus;</button>
              <input
                type="number"
                min={1}
                max={100}
                value={qty}
                aria-label="Quantity"
                data-testid="quantity-input"
                onChange={(e) => setQty(Math.max(1, Math.min(100, Number(e.target.value) || 1)))}
                className="w-14 border-x border-slate-200 py-2 text-center outline-none [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none"
              />
              <button type="button" aria-label="Increase quantity" data-testid="quantity-increase" onClick={() => setQty((q) => Math.min(100, q + 1))} className="px-3 py-2 text-lg text-slate-600 hover:text-slate-900">+</button>
            </div>
            <button
              type="button"
              onClick={onAdd}
              disabled={status === 'busy'}
              data-testid="product-detail-add-to-cart"
              className="flex-1 rounded-xl bg-indigo-600 px-6 py-3 font-semibold text-white shadow-sm transition hover:bg-indigo-700 disabled:opacity-70 sm:flex-none"
            >
              {status === 'busy' ? 'Adding...' : 'Add to cart'}
            </button>
          </div>
          {status === 'added' && (
            <div className="flex items-center justify-between rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800" data-testid="added-to-cart-notice" role="status">
              <span>Added {qty} &times; {p.name} to your cart.</span>
              <Link to="/cart" className="font-semibold underline" data-testid="go-to-cart">View cart</Link>
            </div>
          )}
          {status === 'error' && <ErrorBox message={msg} />}

          <div className="mt-4">
            <h2 className="mb-3 text-lg font-semibold">Specifications</h2>
            <table className="w-full overflow-hidden rounded-xl border border-slate-200 bg-white text-sm" data-testid="product-specs">
              <tbody>
                {p.specs?.map((s) => (
                  <tr key={s.key} className="border-b border-slate-100 last:border-0">
                    <th scope="row" className="w-1/3 bg-slate-50 px-4 py-2.5 text-left font-medium text-slate-600">{s.key}</th>
                    <td className="px-4 py-2.5 text-slate-900">{s.value}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  )
}
