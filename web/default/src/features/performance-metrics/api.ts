import axios from 'axios'

import { api } from '@/lib/api'
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { createServerError } from '@/lib/server-error-message'

import type { PerformanceMetricsData, PerfSummaryAllData } from './types'

export async function getPerfMetricsSummary(
  hours = 24,
  options?: { signal?: AbortSignal; optional?: boolean }
): Promise<PerfSummaryAllData> {
  try {
    const res = await api.get<PerfSummaryAllData>('/api/perf-metrics/summary', {
      params: { hours },
      signal: options?.signal,
      skipErrorHandler: options?.optional,
      skipBusinessError: options?.optional,
      disableDuplicate: options?.optional,
    })
    if (options?.optional && (!res.data.success || !res.data.data)) {
      throw createServerError(res.data, 'Failed to load performance metrics')
    }
    return res.data
  } catch (error) {
    // Optional model-square diagnostics must not trigger the global 500 route.
    // Other summary consumers retain their existing request/error behavior.
    if (
      options?.optional &&
      axios.isAxiosError(error) &&
      !axios.isCancel(error)
    ) {
      throw createServerError(error)
    }
    throw error
  }
}

export async function getPerfMetrics(
  modelName: string,
  hours = 24
): Promise<PerformanceMetricsData> {
  const res = await api.get<PerformanceMetricsData>('/api/perf-metrics', {
    params: {
      model: modelName,
      hours,
    },
  })
  return res.data
}
