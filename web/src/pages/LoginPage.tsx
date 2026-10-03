import { useState, type FormEvent } from 'react'
import { Navigate, useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '../state'
import { ErrorBox } from '../components/ui'

const field = 'mt-1 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100'

export default function LoginPage() {
  const { token, login } = useAuth()
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const next = params.get('next') || '/'
  const safeNext = next.startsWith('/') && !next.startsWith('//') ? next : '/'
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (token && !busy) return <Navigate to={safeNext} replace />

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await login(username, password)
      navigate(safeNext, { replace: true })
    } catch (err) {
      setError((err as Error).message)
      setBusy(false)
    }
  }

  return (
    <div className="mx-auto max-w-md">
      <form onSubmit={onSubmit} className="rounded-3xl border border-slate-200 bg-white p-8 shadow-sm" data-testid="login-form">
        <h1 className="text-2xl font-bold tracking-tight" data-testid="login-heading">Sign in to LoadShop</h1>
        <p className="mt-1 text-sm text-slate-500">Any username and password will do: this is a demo store.</p>
        {error && <div className="mt-4"><ErrorBox message={error} /></div>}
        <label className="mt-6 block text-sm font-medium text-slate-700">Username
          <input required autoFocus className={field} value={username} onChange={(e) => setUsername(e.target.value)} data-testid="login-username" autoComplete="username" />
        </label>
        <label className="mt-4 block text-sm font-medium text-slate-700">Password
          <input required type="password" className={field} value={password} onChange={(e) => setPassword(e.target.value)} data-testid="login-password" autoComplete="current-password" />
        </label>
        <button type="submit" disabled={busy} data-testid="login-submit" className="mt-6 w-full rounded-xl bg-indigo-600 px-6 py-3 font-semibold text-white hover:bg-indigo-700 disabled:opacity-70">
          {busy ? 'Signing in...' : 'Sign in'}
        </button>
      </form>
    </div>
  )
}
