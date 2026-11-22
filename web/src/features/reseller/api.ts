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

export interface ResellerPricingGroup {
  name: string
  description: string
  official_ratio: number
  cost_ratio: number
  discount_percent?: number
  special: boolean
}

export interface ResellerPricingModel {
  model_name: string
  billing_mode?: string
  quota_type: number
  model_ratio: number
  model_price: number
  completion_ratio: number
  cache_ratio?: number | null
  create_cache_ratio?: number | null
  image_ratio?: number | null
  audio_ratio?: number | null
  audio_completion_ratio?: number | null
  video_completion_ratio?: number | null
  enable_groups: string[]
}

export interface ResellerPricingCatalog {
  user_group: string
  user_group_description: string
  groups: ResellerPricingGroup[]
  models: ResellerPricingModel[]
  as_of: number
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
