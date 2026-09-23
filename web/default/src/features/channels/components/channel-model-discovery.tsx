import { useMutation } from '@tanstack/react-query'
import axios from 'axios'
import { Loader2, Sparkles } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { Button } from '@/components/ui/button'
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
  requireServerSuccess,
  createServerError,
} from '@/lib/server-error-message'

import {
  fetchModels,
  fetchUpstreamModels,
  type ChannelModelDiscoveryRequest,
} from '../api'
import { UpstreamModelSelection } from './upstream-model-selection'

type ChannelModelDiscoveryProps = {
  request: ChannelModelDiscoveryRequest
  enabled: boolean
  savedChannelId?: number
  selected: string[]
  existingModels: string[]
  redirectModels: string[]
  redirectSourceModels: string[]
  onChange: (models: string[]) => void
}

export function ChannelModelDiscovery(props: ChannelModelDiscoveryProps) {
  const { t } = useTranslation()
  const controller = useRef<AbortController | null>(null)
  const generation = useRef(0)
  const [result, setResult] = useState<string[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const request = props.request
  const cancelDiscovery = useCallback(() => {
    generation.current++
    controller.current?.abort()
  }, [])

  useEffect(() => {
    cancelDiscovery()
    setResult(null)
    setError(null)
    return cancelDiscovery
  }, [
    request.type,
    request.channel_id,
    request.base_url,
    request.key,
    request.advanced_custom,
    request.header_override,
    request.proxy,
    props.enabled,
    cancelDiscovery,
    props.savedChannelId,
  ])

  const discovery = useMutation({
    // Credentials stay in the form/closure, never in query keys or mutation variables.
    mutationFn: async () => {
      controller.current?.abort()
      const activeController = new AbortController()
      controller.current = activeController
      const activeGeneration = ++generation.current
      try {
        const options = { signal: activeController.signal }
        const response = props.savedChannelId
          ? requireServerSuccess(
              await fetchUpstreamModels(props.savedChannelId, options)
            )
          : requireServerSuccess(await fetchModels(request, options))
        if (
          activeController.signal.aborted ||
          activeGeneration !== generation.current
        ) {
          throw new axios.CanceledError()
        }
        if (!response.success || !response.data) {
          throw createServerError(response, t('Failed to fetch models'))
        }
        return { models: response.data, generation: activeGeneration }
      } catch (failure) {
        if (
          activeController.signal.aborted ||
          activeGeneration !== generation.current
        ) {
          throw new axios.CanceledError()
        }
        throw failure
      }
    },
    retry: false,
    onSuccess: (response) => {
      if (response.generation === generation.current) setResult(response.models)
    },
    onError: (failure) => {
      if (axios.isCancel(failure) || controller.current?.signal.aborted) return
      setError(t('Failed to fetch models'))
    },
  })

  const discover = () => {
    if (!props.enabled || discovery.isPending) return
    if (!request.channel_id && !request.key?.trim()) {
      setError(t('Please enter API key first'))
      return
    }
    setError(null)
    setResult(null)
    discovery.mutate()
  }

  return (
    <section className='space-y-3' aria-label={t('Fetch from Upstream')}>
      <Button
        type='button'
        variant='outline'
        size='sm'
        onClick={discover}
        disabled={!props.enabled || discovery.isPending}
      >
        {discovery.isPending ? (
          <Loader2 className='size-4 animate-spin' aria-hidden='true' />
        ) : (
          <Sparkles className='size-4' aria-hidden='true' />
        )}
        {t('Fetch from Upstream')}
      </Button>
      {!props.enabled && (
        <p className='text-muted-foreground text-xs'>
          {t('No permission to perform this action')}
        </p>
      )}
      {error && (
        <ErrorState
          description={error}
          onRetry={discover}
          className='min-h-0 py-4'
        />
      )}
      {result && (
        <UpstreamModelSelection
          models={result}
          selected={props.selected}
          existingModels={props.existingModels}
          redirectModels={props.redirectModels}
          redirectSourceModels={props.redirectSourceModels}
          onChange={props.onChange}
        />
      )}
    </section>
  )
}
