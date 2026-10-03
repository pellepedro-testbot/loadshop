import { useParams, useSearchParams } from 'react-router-dom'
import type { Product } from '../api'
import { useApi, useCategories } from '../hooks'
import { ErrorBox, ProductCard, Spinner } from '../components/ui'

const SORTS = [
  { value: '', label: 'Featured' },
  { value: 'price_asc', label: 'Price: low to high' },
  { value: 'price_desc', label: 'Price: high to low' },
  { value: 'rating', label: 'Top rated' },
  { value: 'name', label: 'Name' },
]

export default function CatalogPage() {
  const { category } = useParams()
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const sort = params.get('sort') ?? ''
  const categories = useCategories()
  const catName = categories.find((c) => c.slug === category)?.name

  const qs = new URLSearchParams({ limit: '100' })
  if (category) qs.set('category', category)
  if (q) qs.set('q', q)
  if (sort) qs.set('sort', sort)
  const { data, error, loading } = useApi<{ items: Product[]; total: number }>(`/api/products?${qs}`)

  const heading = q ? `Results for "${q}"` : catName ?? (category ? 'Category' : 'All products')

  return (
    <div>
      {!category && !q && (
        <section className="mb-8 overflow-hidden rounded-3xl bg-gradient-to-br from-indigo-600 via-indigo-700 to-slate-900 px-6 py-10 text-white sm:px-10">
          <p className="text-sm font-semibold uppercase tracking-widest text-indigo-200">New season</p>
          <p className="mt-2 max-w-xl text-3xl font-bold tracking-tight sm:text-4xl">Tech that keeps up with you.</p>
          <p className="mt-3 max-w-xl text-indigo-100">Laptops, phones, audio and more, with free shipping over $50 and 30-day returns.</p>
        </section>
      )}

      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-slate-900" data-testid="catalog-heading">{heading}</h1>
          {data && <p className="mt-1 text-sm text-slate-500" data-testid="catalog-count">{data.total} products</p>}
        </div>
        <label className="flex items-center gap-2 text-sm text-slate-600">
          Sort by
          <select
            value={sort}
            data-testid="sort-select"
            onChange={(e) => {
              const next = new URLSearchParams(params)
              if (e.target.value) next.set('sort', e.target.value)
              else next.delete('sort')
              setParams(next)
            }}
            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm outline-none focus:border-indigo-500"
          >
            {SORTS.map((s) => (
              <option key={s.value} value={s.value}>{s.label}</option>
            ))}
          </select>
        </label>
      </div>

      {error && <ErrorBox message={`Could not load products: ${error}`} />}
      {loading && !data && <Spinner label="Loading products" />}
      {data && data.items.length === 0 && (
        <div className="rounded-2xl border border-dashed border-slate-300 bg-white py-16 text-center text-slate-500" data-testid="catalog-empty">
          No products match your search.
        </div>
      )}
      {data && data.items.length > 0 && (
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4" data-testid="product-grid">
          {data.items.map((p) => (
            <ProductCard key={p.id} product={p} />
          ))}
        </div>
      )}
    </div>
  )
}
