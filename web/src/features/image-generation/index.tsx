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
import { ImagePlus } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { handleServerError } from '@/lib/handle-server-error'

import {
  getImageGenerationGroups,
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
  const [group, setGroup] = useState('')
  const [taskId, setTaskId] = useState('')

  const groupsQuery = useQuery({
    queryKey: ['image-generation', 'groups'],
    queryFn: getImageGenerationGroups,
  })
  const modelsQuery = useQuery({
    queryKey: ['image-generation', 'models', group],
    queryFn: () => getImageGenerationModels(group),
    enabled: group !== '',
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
    <div className='bg-muted/20 flex size-full min-h-0 [scrollbar-width:none] flex-col overflow-x-hidden overflow-y-auto [&::-webkit-scrollbar]:hidden'>
      <div className='mx-auto flex w-full max-w-[1600px] flex-1 flex-col gap-4 p-3 sm:gap-5 sm:p-5 lg:p-6'>
        <header className='flex items-start gap-3 px-1 sm:items-center'>
          <div className='bg-primary/10 text-primary flex size-9 shrink-0 items-center justify-center rounded-lg sm:size-10 sm:rounded-xl'>
            <ImagePlus className='size-5' aria-hidden='true' />
          </div>
          <div className='min-w-0'>
            <h1 className='text-xl font-semibold tracking-tight sm:text-2xl'>
              {t('Image Generation')}
            </h1>
            <p className='text-muted-foreground mt-0.5 text-sm sm:mt-1'>
              {t('Create and edit images with Nano Banana and GPT Image.')}
            </p>
          </div>
        </header>

        <div className='grid min-h-0 flex-1 items-start gap-4 sm:gap-5 xl:grid-cols-[minmax(360px,440px)_minmax(0,1fr)] xl:items-stretch'>
          <Card
            data-card-hover='false'
            className='min-w-0 overflow-visible xl:min-h-[calc(100dvh-11rem)]'
          >
            <CardHeader className='border-b px-4 sm:px-6'>
              <CardTitle>{t('Generation settings')}</CardTitle>
            </CardHeader>
            <CardContent className='flex flex-1 flex-col px-4 sm:px-6'>
              <GenerationForm
                disabled={disabled}
                groups={groupsQuery.data ?? []}
                isLoadingModels={modelsQuery.isLoading}
                models={modelsQuery.data ?? []}
                onGroupChange={setGroup}
                onSubmit={(value, references) =>
                  generationMutation.mutate({ references, value })
                }
              />
            </CardContent>
          </Card>

          <Card
            data-card-hover='false'
            className='min-h-[360px] min-w-0 gap-0 overflow-visible py-0 sm:min-h-[480px] xl:min-h-[calc(100dvh-11rem)]'
          >
            <GenerationResult
              history={
                historyQuery.data?.pages.flatMap((page) => page.items) ?? []
              }
              hasMore={historyQuery.hasNextPage}
              isLoadingMore={historyQuery.isFetchingNextPage}
              onLoadMore={() => void historyQuery.fetchNextPage()}
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
