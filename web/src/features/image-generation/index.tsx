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
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useMediaQuery } from '@/hooks/use-media-query'
import { handleServerError } from '@/lib/handle-server-error'

import {
  getImageGenerationHistory,
  getImageGenerationModels,
  getImageGenerationTask,
  submitImageGeneration,
  uploadReferenceImage,
} from './api'
import { GenerationForm } from './components/generation-form'
import { GenerationResult } from './components/generation-result'
import {
  buildImageGenerationRequest,
  isTerminalTaskStatus,
} from './lib/image-generation'
import type { ImageGenerationFormValue, ReferenceImage } from './types'

export function ImageGeneration() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [taskId, setTaskId] = useState('')
  const isCompact = useMediaQuery('(max-width: 1279px)')

  const modelsQuery = useQuery({
    queryKey: ['image-generation', 'models', 'auto'],
    queryFn: () => getImageGenerationModels('auto'),
  })
  const taskQuery = useQuery({
    queryKey: ['image-generation', 'task', taskId],
    queryFn: () => getImageGenerationTask(taskId),
    enabled: taskId !== '',
    refetchInterval: (query) =>
      isTerminalTaskStatus(query.state.data?.status) ? false : 1500,
  })
  const historyQuery = useInfiniteQuery({
    queryKey: ['image-generation', 'history'],
    queryFn: ({ pageParam }) => getImageGenerationHistory(pageParam),
    initialPageParam: 0,
    getNextPageParam: (lastPage) => lastPage.next_before_id || undefined,
  })

  useEffect(() => {
    if (taskQuery.data?.status === 'SUCCESS') {
      void queryClient.invalidateQueries({
        queryKey: ['image-generation', 'history'],
      })
    }
  }, [queryClient, taskQuery.data?.status, taskQuery.data?.task_id])

  const generationMutation = useMutation({
    mutationFn: async (input: {
      references: ReferenceImage[]
      value: ImageGenerationFormValue
    }) => {
      const uploaded = await Promise.all(
        input.references.map((reference) =>
          uploadReferenceImage(reference.file)
        )
      )
      const request = buildImageGenerationRequest(
        input.value,
        uploaded.map((attachment) => attachment.url)
      )
      return submitImageGeneration(request)
    },
    onMutate: () => setTaskId(''),
    onSuccess: (submission) => setTaskId(submission.task_id),
    onError: (error) => handleServerError(error, t('Image generation failed')),
  })

  const taskIsActive =
    taskId !== '' && !isTerminalTaskStatus(taskQuery.data?.status)
  const disabled = generationMutation.isPending || taskIsActive

  return (
    <div className='bg-muted/20 flex size-full min-h-0 flex-col overflow-x-hidden overflow-y-auto overscroll-y-contain xl:overflow-hidden'>
      <div className='flex min-h-full w-full flex-1 flex-col p-2 pb-[calc(1rem+env(safe-area-inset-bottom))] sm:p-3 lg:p-4 xl:min-h-0'>
        <div className='grid w-full auto-rows-max grid-cols-1 content-start gap-3 xl:min-h-0 xl:flex-1 xl:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] xl:grid-rows-[minmax(0,1fr)] xl:auto-rows-auto xl:content-normal xl:items-stretch'>
          <Card
            data-card-hover='false'
            className='min-w-0 self-start overflow-visible xl:h-full xl:min-h-0 xl:self-auto'
          >
            <CardHeader className='border-b px-3 sm:px-6'>
              <CardTitle>{t('Generation settings')}</CardTitle>
            </CardHeader>
            <CardContent className='flex flex-col px-3 sm:px-6 xl:min-h-0 xl:flex-1 xl:overflow-y-auto xl:[scrollbar-width:none] xl:[&::-webkit-scrollbar]:hidden'>
              <GenerationForm
                disabled={disabled}
                isLoadingModels={modelsQuery.isLoading}
                models={modelsQuery.data ?? []}
                onSubmit={(value, references) =>
                  generationMutation.mutate({ references, value })
                }
              />
            </CardContent>
          </Card>

          <Card
            data-card-hover='false'
            className='min-h-[55dvh] min-w-0 self-start gap-0 overflow-visible py-0 xl:h-full xl:min-h-0 xl:self-auto xl:overflow-hidden'
          >
            <GenerationResult
              compact={isCompact}
              history={
                historyQuery.data?.pages.flatMap((page) => page.items) ?? []
              }
              hasMore={historyQuery.hasNextPage}
              isLoadingMore={historyQuery.isFetchingNextPage}
              onLoadMore={async () => {
                const nextPage = await historyQuery.fetchNextPage()
                return !nextPage.isError
              }}
              isError={taskQuery.isError}
              isHistoryError={historyQuery.isError}
              isHistoryLoading={historyQuery.isPending}
              isLoading={generationMutation.isPending}
              onRetry={() => {
                void taskQuery.refetch()
                void historyQuery.refetch()
              }}
              task={taskQuery.data}
            />
          </Card>
        </div>
      </div>
    </div>
  )
}
