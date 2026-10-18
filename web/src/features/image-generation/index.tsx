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
  useQueries,
} from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useMediaQuery } from '@/hooks/use-media-query'
import { handleServerError } from '@/lib/handle-server-error'

import {
  getImageGenerationHistory,
  getImageGenerationModels,
  getImageGenerationTask,
  deleteImageGenerationImages,
  submitImageGeneration,
  uploadReferenceImage,
} from './api'
import { GenerationForm } from './components/generation-form'
import { GenerationResult } from './components/generation-result'
import {
  buildImageGenerationRequest,
  isTerminalTaskStatus,
} from './lib/image-generation'
import type {
  ImageGenerationFormValue,
  ImageGallerySelection,
  ImageResultAction,
  ReferenceImage,
} from './types'

export function ImageGeneration() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [taskIds, setTaskIds] = useState<string[]>([])
  const [editDraft, setEditDraft] = useState<{
    key: number
    image: ImageResultAction
  }>()
  const [pendingRegenerate, setPendingRegenerate] = useState<
    ImageResultAction[]
  >([])
  const draftKeyRef = useRef(0)
  const settingsCardRef = useRef<HTMLDivElement>(null)
  const isCompact = useMediaQuery('(max-width: 1279px)')

  const modelsQuery = useQuery({
    queryKey: ['image-generation', 'models', 'auto'],
    queryFn: () => getImageGenerationModels('auto'),
  })
  const taskQueries = useQueries({
    queries: taskIds.map((taskId) => ({
      queryKey: ['image-generation', 'task', taskId],
      queryFn: () => getImageGenerationTask(taskId),
      refetchInterval: (query: {
        state: { data?: { status?: string }; status: string }
      }) => {
        if (
          query.state.status === 'error' ||
          isTerminalTaskStatus(query.state.data?.status)
        ) {
          return false
        }
        return taskIds.length > 20 ? 8000 : 4000
      },
    })),
  })
  const tasks = taskQueries.flatMap((query) => (query.data ? [query.data] : []))
  const finishedTaskIds = tasks
    .filter((task) => task.status === 'SUCCESS')
    .map((task) => task.task_id)
    .join(',')
  const historyQuery = useInfiniteQuery({
    queryKey: ['image-generation', 'history'],
    queryFn: ({ pageParam }) => getImageGenerationHistory(pageParam),
    initialPageParam: 0,
    getNextPageParam: (lastPage) => lastPage.next_before_id || undefined,
  })

  useEffect(() => {
    if (finishedTaskIds) {
      void queryClient.invalidateQueries({
        queryKey: ['image-generation', 'history'],
      })
    }
  }, [queryClient, finishedTaskIds])

  const generationMutation = useMutation({
    mutationFn: async (input: {
      referenceSets: ReferenceImage[][]
      value: ImageGenerationFormValue
    }) => {
      const uniqueReferences = new Map<string, ReferenceImage>()
      for (const set of input.referenceSets) {
        for (const reference of set) {
          uniqueReferences.set(reference.id, reference)
        }
      }
      const uploadedURLs = new Map<string, string>()
      const references = [...uniqueReferences.values()]
      for (let index = 0; index < references.length; index += 3) {
        const chunk = references.slice(index, index + 3)
        const uploaded = await Promise.all(
          chunk.map((reference) => {
            if (reference.sourceUrl) {
              return { url: reference.sourceUrl }
            }
            if (!reference.file) throw new Error('Missing reference image file')
            return uploadReferenceImage(reference.file)
          })
        )
        chunk.forEach((reference, chunkIndex) => {
          uploadedURLs.set(reference.id, uploaded[chunkIndex].url)
        })
      }

      const submittedIDs: string[] = []
      for (const set of input.referenceSets) {
        const request = buildImageGenerationRequest(
          input.value,
          set.map((reference) => uploadedURLs.get(reference.id) ?? '')
        )
        const submission = await submitImageGeneration(request)
        submittedIDs.push(submission.task_id)
        setTaskIds([...submittedIDs])
      }
      return submittedIDs
    },
    onMutate: () => setTaskIds([]),
    onError: (error) => handleServerError(error, t('Image generation failed')),
  })

  const regenerateMutation = useMutation({
    mutationFn: async (images: ImageResultAction[]) => {
      const submittedIDs: string[] = []
      for (const image of images) {
        const options = image.options
        const formValue: ImageGenerationFormValue = {
          aspectRatio: options?.aspect_ratio || '1:1',
          background:
            options?.background === 'transparent' ? 'transparent' : 'auto',
          model: image.model,
          outputFormat: 'png',
          prompt: image.prompt,
          quality: options?.quality || 'auto',
          size:
            options?.size ||
            (image.model.startsWith('gemini-') ? '1K' : 'auto'),
          thinkingLevel:
            options?.thinking_level === 'high' ||
            options?.thinking_level === 'minimal'
              ? options.thinking_level
              : 'default',
        }
        const outputFormat = options?.output_format ?? image.outputFormat
        if (outputFormat === 'jpeg' || outputFormat === 'webp') {
          formValue.outputFormat = outputFormat
        }
        const submission = await submitImageGeneration(
          buildImageGenerationRequest(formValue, [image.url])
        )
        submittedIDs.push(submission.task_id)
        setTaskIds([...submittedIDs])
      }
      return submittedIDs
    },
    onMutate: () => setTaskIds([]),
    onError: (error) => handleServerError(error, t('Image generation failed')),
  })

  const taskIsActive = taskIds.some(
    (_, index) =>
      !taskQueries[index]?.isError &&
      !isTerminalTaskStatus(taskQueries[index]?.data?.status)
  )
  const disabled =
    generationMutation.isPending || regenerateMutation.isPending || taskIsActive

  const deleteGalleryImages = async (items: ImageGallerySelection[]) => {
    await deleteImageGenerationImages(items)
    const affectedTaskIds = new Set(items.map((item) => item.task_id))
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: ['image-generation', 'history'],
      }),
      ...[...affectedTaskIds].map((taskId) =>
        queryClient.invalidateQueries({
          queryKey: ['image-generation', 'task', taskId],
        })
      ),
    ])
  }

  const regenerateImages = (images: ImageResultAction[]) => {
    if (disabled || images.length === 0) return
    if (images.length > 10) {
      setPendingRegenerate(images)
      return
    }
    regenerateMutation.mutate(images)
  }

  return (
    <div className='bg-muted/20 flex size-full min-h-0 flex-col overflow-x-hidden overflow-y-auto overscroll-y-contain xl:overflow-hidden'>
      <div className='flex min-h-full w-full flex-1 flex-col p-2 pb-[calc(1rem+env(safe-area-inset-bottom))] sm:p-3 lg:p-4 xl:min-h-0'>
        <div className='grid w-full auto-rows-max grid-cols-1 content-start gap-3 xl:min-h-0 xl:flex-1 xl:auto-rows-auto xl:grid-cols-[minmax(0,3fr)_minmax(0,7fr)] xl:grid-rows-[minmax(0,1fr)] xl:content-normal xl:items-stretch'>
          <Card
            ref={settingsCardRef}
            data-card-hover='false'
            className='min-w-0 self-start overflow-visible xl:h-full xl:min-h-0 xl:self-auto'
          >
            <CardHeader className='border-b px-3 sm:px-6'>
              <CardTitle>{t('Generation settings')}</CardTitle>
            </CardHeader>
            <CardContent className='@container flex flex-col px-3 sm:px-6 xl:min-h-0 xl:flex-1 xl:[scrollbar-width:none] xl:overflow-y-auto xl:[&::-webkit-scrollbar]:hidden'>
              <GenerationForm
                disabled={disabled}
                editDraft={editDraft}
                isLoadingModels={modelsQuery.isLoading}
                models={modelsQuery.data ?? []}
                onSubmit={(value, referenceSets) =>
                  generationMutation.mutate({ referenceSets, value })
                }
              />
            </CardContent>
          </Card>

          <Card
            data-card-hover='false'
            className='min-h-[55dvh] min-w-0 gap-0 self-start overflow-visible py-0 xl:h-full xl:min-h-0 xl:self-auto xl:overflow-hidden'
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
              isError={taskQueries.some((query) => query.isError)}
              isHistoryError={historyQuery.isError}
              isHistoryLoading={historyQuery.isPending}
              isLoading={
                generationMutation.isPending || regenerateMutation.isPending
              }
              onDelete={deleteGalleryImages}
              onEdit={(image) => {
                draftKeyRef.current++
                setEditDraft({ key: draftKeyRef.current, image })
                settingsCardRef.current?.scrollIntoView({
                  behavior: 'smooth',
                  block: 'start',
                })
              }}
              onRegenerate={regenerateImages}
              onRetry={() => {
                for (const query of taskQueries) void query.refetch()
                void historyQuery.refetch()
              }}
              tasks={tasks}
              pendingTaskIds={taskIds.filter(
                (_, index) => !taskQueries[index]?.isError
              )}
              actionsDisabled={disabled}
              taskKey={taskIds[0]}
            />
          </Card>
        </div>
      </div>
      <ConfirmDialog
        open={pendingRegenerate.length > 0}
        onOpenChange={(open) => {
          if (!open) setPendingRegenerate([])
        }}
        title={t('Confirm batch generation')}
        desc={t('This will submit {{count}} separately billed image tasks.', {
          count: pendingRegenerate.length,
        })}
        confirmText={t('Regenerate selected')}
        handleConfirm={() => {
          regenerateMutation.mutate(pendingRegenerate)
          setPendingRegenerate([])
        }}
      />
    </div>
  )
}
