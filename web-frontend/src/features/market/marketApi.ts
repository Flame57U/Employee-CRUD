import { apiRequest } from '@/lib/api'

// Mirrors internal/saas/itick.Quote (readable JSON tags from the Go struct).
export interface Quote {
  s: string // symbol
  ld: number // last price
  o: number // open
  h: number // high
  l: number // low
  t: number // epoch ms
  v: number // volume
  tu: number // turnover
  type: string
  r: string // region
  ch: number // change
  chp: number // change percent
}

export type Asset = 'forex' | 'crypto' | 'stock'

export function getQuote(asset: Asset, region: string, code: string) {
  const qs = new URLSearchParams({ asset, region, code }).toString()
  return apiRequest<Quote>(`/market/quote?${qs}`)
}
