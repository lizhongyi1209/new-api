import type { UsageLog } from '@/features/usage-logs/data/schema'
import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

export interface Customer {
  id: number
  username: string
  quota: number
  used_quota: number
  created_at: number
  last_login_at: number
  user_group: string
}

export interface Bootstrap {
  customers: Customer[]
  models: string[]
  groups: string[]
  as_of: number
}

export interface PricingGroup {
  user_group: string
  billing_group: string
  net_quota: number
}

export interface Period {
  spent_quota: number
  topup_quota: number
  refund_quota: number
  remaining_quota: number
  used_quota: number
  estimated_balance: boolean
  as_of: number
  pricing_groups: PricingGroup[]
}

export interface LogPage {
  rows: UsageLog[]
  total: number
  page: number
}
export interface LogFilters {
  user: string
  model: string
  group: string
  type: string
  token: string
  request: string
}
export const emptyLogFilters: LogFilters = {
  user: '',
  model: '',
  group: '',
  type: '',
  token: '',
  request: '',
}

export async function resellerGet<T>(
  path: string,
  params?: Record<string, string | number>,
  signal?: AbortSignal
): Promise<T> {
  const result = await api.get(`/api/reseller/${path}`, { params, signal })
  return requireServerSuccess(result.data).data as T
}

export async function setResellerAccess(
  username: string,
  enabled: boolean
): Promise<void> {
  const result = await api.put('/api/reseller/access', { username, enabled })
  requireServerSuccess(result.data)
}
