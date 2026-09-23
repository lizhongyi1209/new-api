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
import { useQuery } from '@tanstack/react-query'
import { t } from 'i18next'

import { fetchTokenKey, getApiKeys } from '@/features/keys/api'
import { API_KEY_STATUS } from '@/features/keys/constants'
import { createServerError } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

export async function fetchActiveChatKey() {
  const initialAuth = useAuthStore.getState().auth
  const userId = initialAuth.user?.id
  const sessionId = initialAuth.session?.sid
  if (!userId) throw new Error(t('Session expired!'))
  const result = await getApiKeys({ p: 1, size: 50 })
  if (!result.success) {
    throw createServerError(result, t('Failed to load API keys'))
  }

  const items = result.data?.items ?? []
  const active = items.find((item) => item.status === API_KEY_STATUS.ENABLED)
  if (!active) {
    throw new Error(t('No enabled API keys found. Create or enable one first.'))
  }

  const currentAuth = useAuthStore.getState().auth
  if (
    currentAuth.user?.id !== userId ||
    currentAuth.session?.sid !== sessionId
  ) {
    throw new Error(t('Session expired!'))
  }
  const keyResult = await fetchTokenKey(active.id)
  const resolvedAuth = useAuthStore.getState().auth
  if (
    resolvedAuth.user?.id !== userId ||
    resolvedAuth.session?.sid !== sessionId
  ) {
    throw new Error(t('Session expired!'))
  }
  if (!keyResult.success || !keyResult.data?.key) {
    throw createServerError(keyResult, t('Failed to load API keys'))
  }

  const key = keyResult.data.key
  return key.startsWith('sk-') ? key : `sk-${key}`
}

/**
 * Get the currently active API key for chat links
 */
export function useActiveChatKey(enabled: boolean) {
  const userId = useAuthStore((state) => state.auth.user?.id)

  return useQuery({
    queryKey: ['chat-active-key', userId],
    queryFn: fetchActiveChatKey,
    enabled: enabled && Boolean(userId),
    staleTime: 5 * 60 * 1000,
    gcTime: 10 * 60 * 1000,
  })
}
