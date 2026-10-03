import { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { money, type Product } from '../api'
import { useAuth, useCart } from '../state'

export function Stars({ rating, count, testId }: { rating: number; count?: number; testId?: string }) {
  return (
    <div className="flex items-center gap-1.5 text-sm" data-testid={testId} aria-label={`Rated ${rating} out of 5`}>
      <div className="flex">
        {[1, 2, 3, 4, 5].map((i) => {
          const fill = Math.max(0, Math.min(1, rating - (i - 1)))
          return (
            <span key={i} className="relative h-4 w-4">
              <svg viewBox="0 0 20 20" className="absolute h-4 w-4 text-slate-200" fill="currentColor"><path d="M10 1.5l2.6 5.4 5.9.8-4.3 4.1 1 5.9L10 14.9l-5.2 2.8 1-5.9L1.5 7.7l5.9-.8L10 1.5z" /></svg>
              <span className="absolute overflow-hidden" style={{ width: `${fill * 100}%` }}>
                <svg viewBox="0 0 20 20" className="h-4 w-4 text-amber-400" fill="currentColor"><path d="M10 1.5l2.6 5.4 5.9.8-4.3 4.1 1 5.9L10 14.9l-5.2 2.8 1-5.9L1.5 7.7l5.9-.8L10 1.5z" /></svg>
              </span>
            </span>
          )
        })}
      </div>
      <span className="font-medium text-slate-700">{rating.toFixed(1)}</span>
      {count !== undefined && <span className="text-slate-400">({count.toLocaleString()})</span>}
    </div>
  )
}

export function Spinner({ label = 'Loading' }: { label?: string }) {
  return (
    <div className="flex items-center justify-center gap-3 py-24 text-slate-500" data-testid="loading">
      <span className="h-5 w-5 animate-spin rounded-full border-2 border-slate-300 border-t-indigo-600" />
      {label}
    </div>
  )
}

export function ErrorBox({ message }: { message: string }) {
  return (
    <div className="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700" data-testid="error-message" role="alert">
      {message}
    </div>
  )
}

/** Adds to cart, redirecting to login first when signed out. */
export function useAddToCart() {
  const { token } = useAuth()
  const { add } = useCart()
  const navigate = useNavigate()
  const loc = useLocation()
  return async (productId: number, quantity = 1) => {
    if (!token) {
      navigate(`/login?next=${encodeURIComponent(loc.pathname + loc.search)}`)
      return false
    }
    await add(productId, quantity)
    return true
  }
}

export function ProductCard({ product: p }: { product: Product }) {
  const addToCart = useAddToCart()
  const [state, setState] = useState<'idle' | 'busy' | 'added' | 'error'>('idle')

  const onAdd = async () => {
    setState('busy')
    try {
      const ok = await addToCart(p.id)
      setState(ok ? 'added' : 'idle')
      if (ok) setTimeout(() => setState('idle'), 1500)
    } catch {
      setState('error')
    }
  }

  return (
    <article
      data-testid={`product-card-${p.slug}`}
      className="group flex flex-col overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm transition hover:-translate-y-0.5 hover:shadow-md"
    >
      <Link to={`/product/${p.slug}`} className="block aspect-[4/3] overflow-hidden bg-slate-100" tabIndex={-1} aria-hidden="true">
        <img src={p.image} alt="" loading="lazy" width={600} height={450} className="h-full w-full object-cover transition duration-300 group-hover:scale-[1.03]" />
      </Link>
      <div className="flex flex-1 flex-col gap-2 p-4">
        <div className="flex items-center justify-between text-xs font-medium uppercase tracking-wide">
          <span className="text-indigo-600" data-testid={`product-brand-${p.slug}`}>{p.brand}</span>
          <span className="text-slate-400">{p.category}</span>
        </div>
        <h2 className="text-base font-semibold leading-snug text-slate-900" data-testid={`product-name-${p.slug}`}>
          {p.name}
        </h2>
        <Stars rating={p.rating} count={p.reviewCount} testId={`product-rating-${p.slug}`} />
        <p className="line-clamp-2 text-sm text-slate-500">{p.shortDescription}</p>
        <div className="mt-auto flex items-end justify-between pt-2">
          <span className="text-xl font-bold text-slate-900" data-testid={`product-price-${p.slug}`}>{money(p.price)}</span>
        </div>
        <div className="grid grid-cols-2 gap-2">
          <Link
            to={`/product/${p.slug}`}
            data-testid={`view-details-${p.slug}`}
            className="rounded-lg border border-slate-300 px-3 py-2 text-center text-sm font-medium text-slate-700 hover:border-slate-400 hover:bg-slate-50"
          >
            View details
          </Link>
          <button
            type="button"
            onClick={onAdd}
            disabled={state === 'busy'}
            data-testid={`add-to-cart-${p.slug}`}
            className={`rounded-lg px-3 py-2 text-sm font-medium text-white transition ${
              state === 'added' ? 'bg-emerald-600' : state === 'error' ? 'bg-rose-600' : 'bg-indigo-600 hover:bg-indigo-700'
            } disabled:opacity-70`}
          >
            {state === 'added' ? 'Added' : state === 'error' ? 'Retry' : 'Add to cart'}
          </button>
        </div>
      </div>
    </article>
  )
}
