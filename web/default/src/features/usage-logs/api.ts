import axios, { type AxiosResponse } from 'axios'

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
import {
  createServerError,
  requireServerSuccess,
} from '@/lib/server-error-message'

import { buildQueryParams } from './lib/query'
import type {
  GetLogsParams,
  GetLogsResponse,
  GetLogStatsParams,
  GetLogStatsResponse,
  GetMidjourneyLogsParams,
  GetTaskLogsParams,
  ExportUsageLogsParams,
  UsageLogExportOptions,
  UserInfo,
} from './types'

// ============================================================================
// Generic API Helpers
// ============================================================================

function buildApiPath(endpoint: string, isAdmin: boolean): string {
  return isAdmin ? endpoint : `${endpoint}/self`
}

async function fetchLogs<T>(
  endpoint: string,
  params: T,
  isAdmin: boolean
): Promise<GetLogsResponse> {
  const paramRecord = params as unknown as Record<string, unknown>
  const queryParams = buildQueryParams({
    p: paramRecord.p || 1,
    page_size: paramRecord.page_size || 20,
    ...params,
  })
  const path = buildApiPath(endpoint, isAdmin)
  const res = await api.get(`${path}?${queryParams}`)
  return res.data
}

async function fetchLogStats<T>(
  endpoint: string,
  params: T,
  isAdmin: boolean
): Promise<GetLogStatsResponse> {
  const queryParams = buildQueryParams(
    params as unknown as Record<string, unknown>
  )
  const path = buildApiPath(endpoint, isAdmin)
  const res = await api.get(`${path}/stat?${queryParams}`)
  return res.data
}

// ============================================================================
// Common Log APIs
// ============================================================================

export const getAllLogs = (params: GetLogsParams = {}) =>
  fetchLogs('/api/log', params, true)

export const getUserLogs = (
  params: Omit<GetLogsParams, 'username' | 'channel'> = {}
) => fetchLogs('/api/log', params, false)

export const getLogStats = (params: GetLogStatsParams = {}) =>
  fetchLogStats('/api/log', params, true)

export const getUserLogStats = (
  params: Omit<GetLogStatsParams, 'username' | 'channel'> = {}
) => fetchLogStats('/api/log', params, false)

export async function exportUsageLogs(
  params: ExportUsageLogsParams,
  isAdmin: boolean
): Promise<{ blob: Blob; filename: string }> {
  const path = isAdmin ? '/api/log/export' : '/api/log/self/export'
  const queryParams = buildQueryParams(
    params as unknown as Record<string, unknown>
  )
  let response: AxiosResponse<Blob>
  try {
    response = await api.get(`${path}?${queryParams}`, {
      responseType: 'blob',
      skipErrorHandler: true,
    })
  } catch (error) {
    if (axios.isAxiosError(error) && error.response?.data instanceof Blob) {
      const body = await error.response.data.text()
      try {
        const parsed = JSON.parse(body) as { message?: string }
        throw new Error(parsed.message || 'Failed to export logs')
      } catch (parseError) {
        if (parseError instanceof SyntaxError) {
          throw new Error('Failed to export logs')
        }
        throw parseError
      }
    }
    throw error
  }
  const disposition = String(response.headers['content-disposition'] || '')
  const filenameMatch = disposition.match(/filename="?([^";]+)"?/i)
  return {
    blob: response.data as Blob,
    filename: filenameMatch?.[1] || `usage-logs.${params.format}`,
  }
}

export async function getUsageLogExportOptions(params: {
  start_timestamp: number
  end_timestamp: number
  username?: string
}): Promise<UsageLogExportOptions> {
  const queryParams = buildQueryParams(params)
  const response = await api.get(`/api/log/export/options?${queryParams}`)
  return requireServerSuccess(response.data).data
}

export async function getUserInfo(
  userId: number
): Promise<{ success: boolean; message?: string; data?: UserInfo }> {
  const res = await api.get(`/api/user/${userId}`)
  return res.data
}

// ============================================================================
// MjProxy (Drawing) Logs API
// ============================================================================

export const getAllMidjourneyLogs = (params: GetMidjourneyLogsParams) =>
  fetchLogs('/api/mj', params, true)

export const getUserMidjourneyLogs = (params: GetMidjourneyLogsParams) =>
  fetchLogs('/api/mj', params, false)

// ============================================================================
// Task Logs API
// ============================================================================

export const getAllTaskLogs = (params: GetTaskLogsParams) =>
  fetchLogs('/api/task', params, true)

export const getUserTaskLogs = (params: GetTaskLogsParams) =>
  fetchLogs('/api/task', params, false)

export interface TaskArtifact {
  key: string
  type: 'video' | 'audio' | 'image' | 'file'
  mime_type?: string
  content_url: string
}

export async function getTaskArtifacts(
  taskId: string,
  signal?: AbortSignal
): Promise<TaskArtifact[]> {
  const response = await api.get<{
    success: boolean
    message?: string
    data?: { task_id: string; artifacts: TaskArtifact[] }
  }>(`/api/task/${encodeURIComponent(taskId)}/artifacts`, {
    signal,
    skipErrorHandler: true,
    skipBusinessError: true,
    disableDuplicate: true,
  })
  if (!response.data.success || !response.data.data) {
    throw createServerError(response.data, 'Failed to load artifacts')
  }
  return response.data.data.artifacts ?? []
}

export interface TaskAuditDetails {
  schema_version: number
  task_id: string
  request_id?: string
  upstream_task_id?: string
  channel_id: number
  quota: number
  start_time?: number
}

/** Existing administrator-only endpoint; never requested by self log views. */
export async function getTaskAuditDetails(
  taskId: string,
  signal?: AbortSignal
): Promise<TaskAuditDetails> {
  try {
    const response = await api.get<{
      success: boolean
      message?: string
      data?: TaskAuditDetails
    }>(`/api/task/${encodeURIComponent(taskId)}/audit`, {
      signal,
      skipErrorHandler: true,
      skipBusinessError: true,
      // React Query owns cancellation and deduplication for this query. A remount
      // must not inherit an aborted request from the shared HTTP GET cache.
      disableDuplicate: true,
    })
    if (!response.data.success || !response.data.data) {
      throw createServerError(response.data, 'Failed to load task details')
    }
    return response.data.data
  } catch (error) {
    // Optional diagnostics stay in the detail dialog rather than triggering
    // the global Axios 500 route. Authentication refresh still runs normally.
    if (axios.isAxiosError(error) && !axios.isCancel(error)) {
      throw createServerError(error)
    }
    throw error
  }
}
