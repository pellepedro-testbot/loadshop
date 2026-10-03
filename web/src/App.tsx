import type { ReactNode } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import Header from './components/Header'
import { useAuth } from './state'
import CatalogPage from './pages/CatalogPage'
import ProductPage from './pages/ProductPage'
import CartPage from './pages/CartPage'
import CheckoutPage from './pages/CheckoutPage'
import ConfirmationPage from './pages/ConfirmationPage'
import OrdersPage from './pages/OrdersPage'
import LoginPage from './pages/LoginPage'

function RequireAuth({ children }: { children: ReactNode }) {
  const { token } = useAuth()
  const loc = useLocation()
  if (!token) return <Navigate to={`/login?next=${encodeURIComponent(loc.pathname + loc.search)}`} replace />
  return <>{children}</>
}

export default function App() {
  return (
    <div className="flex min-h-screen flex-col">
      <Header />
      <main className="mx-auto w-full max-w-7xl flex-1 px-4 py-8 sm:px-6 lg:px-8">
        <Routes>
          <Route path="/" element={<CatalogPage />} />
          <Route path="/category/:category" element={<CatalogPage />} />
          <Route path="/product/:slug" element={<ProductPage />} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/cart" element={<RequireAuth><CartPage /></RequireAuth>} />
          <Route path="/checkout" element={<RequireAuth><CheckoutPage /></RequireAuth>} />
          <Route path="/orders" element={<RequireAuth><OrdersPage /></RequireAuth>} />
          <Route path="/orders/:id" element={<RequireAuth><ConfirmationPage /></RequireAuth>} />
          <Route path="*" element={
            <div className="py-24 text-center" data-testid="not-found">
              <h1 className="text-2xl font-semibold">Page not found</h1>
            </div>
          } />
        </Routes>
      </main>
      <footer className="border-t border-slate-200 bg-white">
        <div className="mx-auto flex max-w-7xl flex-col gap-2 px-4 py-6 text-sm text-slate-500 sm:flex-row sm:justify-between sm:px-6 lg:px-8">
          <span>&copy; LoadShop. A demo store for load testing.</span>
          <span>Free shipping on orders over $50 &middot; 30-day returns</span>
        </div>
      </footer>
    </div>
  )
}
