import { useEffect, useRef, useState, type FormEvent } from 'react'
import { Link, NavLink, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth, useCart } from '../state'
import { useCategories } from '../hooks'

export default function Header() {
  const { username, token, logout } = useAuth()
  const { count } = useCart()
  const categories = useCategories()
  const navigate = useNavigate()
  const loc = useLocation()
  const [params] = useSearchParams()
  const [q, setQ] = useState(params.get('q') ?? '')
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => setQ(params.get('q') ?? ''), [params])
  useEffect(() => setMenuOpen(false), [loc.pathname])
  useEffect(() => {
    const close = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) setMenuOpen(false)
    }
    document.addEventListener('mousedown', close)
    return () => document.removeEventListener('mousedown', close)
  }, [])

  const onSearch = (e: FormEvent) => {
    e.preventDefault()
    const term = q.trim()
    navigate(term ? `/?q=${encodeURIComponent(term)}` : '/')
  }

  const navCls = ({ isActive }: { isActive: boolean }) =>
    `whitespace-nowrap rounded-full px-3 py-1.5 text-sm font-medium transition ${
      isActive ? 'bg-indigo-600 text-white' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'
    }`

  return (
    <header className="sticky top-0 z-30 border-b border-slate-200 bg-white/95 backdrop-blur">
      <div className="mx-auto flex max-w-7xl items-center gap-4 px-4 py-3 sm:px-6 lg:px-8">
        <Link to="/" className="flex items-center gap-2" data-testid="logo-link" aria-label="LoadShop home">
          <span className="grid h-9 w-9 place-items-center rounded-xl bg-indigo-600 text-white shadow-sm">
            <svg viewBox="0 0 24 24" className="h-5 w-5" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round"><path d="M3 5h2l2.4 10.5h9.8L20 8H6.3" /><circle cx="9" cy="19.5" r="1.3" /><circle cx="17" cy="19.5" r="1.3" /></svg>
          </span>
          <span className="text-xl font-bold tracking-tight">Load<span className="text-indigo-600">Shop</span></span>
        </Link>

        <form onSubmit={onSearch} className="relative hidden flex-1 sm:block" role="search">
          <input
            type="search"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search products, brands and categories"
            aria-label="Search products"
            data-testid="search-input"
            className="w-full rounded-full border border-slate-300 bg-slate-50 py-2 pl-10 pr-24 text-sm outline-none transition focus:border-indigo-500 focus:bg-white focus:ring-2 focus:ring-indigo-100"
          />
          <svg viewBox="0 0 24 24" className="pointer-events-none absolute left-3.5 top-2.5 h-4 w-4 text-slate-400" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
          <button type="submit" data-testid="search-submit" className="absolute right-1 top-1 rounded-full bg-indigo-600 px-4 py-1 text-sm font-medium text-white hover:bg-indigo-700">
            Search
          </button>
        </form>

        <nav className="ml-auto flex items-center gap-1 sm:ml-0">
          <Link to="/cart" data-testid="cart-link" aria-label={`Cart with ${count} items`} className="relative rounded-full p-2 text-slate-700 hover:bg-slate-100">
            <svg viewBox="0 0 24 24" className="h-6 w-6" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M6 7h12l-1 13H7L6 7Z" /><path d="M9 7a3 3 0 0 1 6 0" /></svg>
            <span data-testid="cart-count" className={`absolute -right-0.5 -top-0.5 grid h-5 min-w-5 place-items-center rounded-full px-1 text-[11px] font-bold ${count > 0 ? 'bg-rose-500 text-white' : 'bg-slate-200 text-slate-600'}`}>
              {count}
            </span>
          </Link>

          {token ? (
            <div className="relative" ref={menuRef}>
              <button
                type="button"
                data-testid="user-menu"
                onClick={() => setMenuOpen((o) => !o)}
                aria-expanded={menuOpen}
                className="flex items-center gap-2 rounded-full py-1 pl-1 pr-3 text-sm font-medium text-slate-700 hover:bg-slate-100"
              >
                <span className="grid h-8 w-8 place-items-center rounded-full bg-indigo-100 font-semibold uppercase text-indigo-700">
                  {(username ?? '?').slice(0, 1)}
                </span>
                <span className="hidden max-w-32 truncate md:inline" data-testid="user-name">{username ?? 'Account'}</span>
              </button>
              {menuOpen && (
                <div className="absolute right-0 mt-2 w-48 overflow-hidden rounded-xl border border-slate-200 bg-white py-1 shadow-lg" role="menu">
                  <Link to="/orders" data-testid="orders-link" className="block px-4 py-2 text-sm hover:bg-slate-50" role="menuitem">My orders</Link>
                  <button
                    type="button"
                    data-testid="logout-button"
                    onClick={() => { logout(); navigate('/') }}
                    className="block w-full px-4 py-2 text-left text-sm text-rose-600 hover:bg-slate-50"
                    role="menuitem"
                  >
                    Sign out
                  </button>
                </div>
              )}
            </div>
          ) : (
            <Link to="/login" data-testid="login-link" className="rounded-full bg-slate-900 px-4 py-2 text-sm font-medium text-white hover:bg-slate-700">
              Sign in
            </Link>
          )}
        </nav>
      </div>

      <div className="mx-auto max-w-7xl overflow-x-auto px-4 pb-3 sm:px-6 lg:px-8">
        <div className="flex gap-1">
          <NavLink to="/" end className={navCls} data-testid="category-link-all">All products</NavLink>
          {categories.map((c) => (
            <NavLink key={c.slug} to={`/category/${c.slug}`} className={navCls} data-testid={`category-link-${c.slug}`}>
              {c.name}
            </NavLink>
          ))}
        </div>
      </div>
    </header>
  )
}
