import { useEffect, useState } from 'react'
import { api, type Category } from './api'

let categoriesCache: Category[] | null = null

export function useCategories() {
  const [cats, setCats] = useState<Category[]>(categoriesCache ?? [])
  useEffect(() => {
    if (categoriesCache) return
    api<Category[]>('/api/categories')
      .then((c) => {
        categoriesCache = c
        setCats(c)
      })
      .catch(() => {})
  }, [])
  return cats
}

export interface Async<T> {
  data: T | null
  error: string | null
  loading: boolean
}

export function useApi<T>(path: string | null): Async<T> {
  const [state, setState] = useState<Async<T>>({ data: null, error: null, loading: !!path })
  useEffect(() => {
    if (!path) return
    let alive = true
    // Drop the previous path's data so a page never shows old content under a new
    // heading while the new response is in flight.
    setState({ data: null, error: null, loading: true })
    api<T>(path)
      .then((data) => alive && setState({ data, error: null, loading: false }))
      .catch((e: Error) => alive && setState({ data: null, error: e.message, loading: false }))
    return () => {
      alive = false
    }
  }, [path])
  return state
}
