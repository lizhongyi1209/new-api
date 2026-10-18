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
  Download,
  Ellipsis,
  ImageIcon,
  LoaderCircle,
  Pencil,
  RotateCw,
  Trash2,
  WandSparkles,
  X,
} from 'lucide-react'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Skeleton } from '@/components/ui/skeleton'
import { Slider } from '@/components/ui/slider'
import { handleServerError } from '@/lib/handle-server-error'
import { cn } from '@/lib/utils'

import { getImageGenerationDownload } from '../api'
import { createImageDownloadArchive } from '../lib/download-archive'
import {
  calculateImageGridLayout,
  generatedImageSource,
  isTerminalTaskStatus,
} from '../lib/image-generation'
import type {
  GeneratedImage,
  ImageGallerySelection,
  ImageGenerationHistoryItem,
  ImageGenerationTask,
  ImageResultAction,
} from '../types'

type GenerationResultProps = {
  actionsDisabled?: boolean
  compact?: boolean
  history: ImageGenerationHistoryItem[]
  hasMore: boolean
  isError: boolean
  isHistoryError: boolean
  isHistoryLoading: boolean
  isLoading: boolean
  isLoadingMore: boolean
  onDelete?: (items: ImageGallerySelection[]) => Promise<void>
  onEdit?: (image: ImageResultAction) => void
  onLoadMore: () => Promise<boolean>
  onRegenerate?: (images: ImageResultAction[]) => void
  onRetry: () => void
  pendingTaskIds?: string[]
  task?: ImageGenerationTask
  taskKey?: string
  tasks?: ImageGenerationTask[]
}

export function GenerationResult(props: GenerationResultProps) {
  const { t } = useTranslation()
  const [selectedTileSize, setTileSize] = useState<number | null>(null)
  const tileSize = selectedTileSize ?? (props.compact ? 130 : 220)
  const [page, setPage] = useState(0)
  const [downloadingImage, setDownloadingImage] = useState<string | null>(null)
  const [selectedImageKeys, setSelectedImageKeys] = useState<Set<string>>(
    () => new Set()
  )
  const [pendingDelete, setPendingDelete] = useState<ImageGallerySelection[]>(
    []
  )
  const [isDeleting, setIsDeleting] = useState(false)
  const [layout, setLayout] = useState({ columns: 2, capacity: 6 })
  const gridRef = useRef<HTMLDivElement>(null)
  const tasks = props.tasks ?? (props.task ? [props.task] : [])
  const activeTaskCount = props.pendingTaskIds
    ? props.pendingTaskIds.filter((taskId) =>
        tasks.every(
          (task) =>
            task.task_id !== taskId || !isTerminalTaskStatus(task.status)
        )
      ).length
    : tasks.filter((task) => !isTerminalTaskStatus(task.status)).length
  const previewCount = Math.max(activeTaskCount, props.isLoading ? 1 : 0)
  const taskIsActive = previewCount > 0
  const failedTask = tasks.find((task) => task.status === 'FAILURE')
  const items = [...props.history]
  for (const task of tasks) {
    if (
      task.status === 'SUCCESS' &&
      task.data?.images?.length &&
      !items.some((item) => item.task_id === task.task_id)
    ) {
      items.unshift({
        task_id: task.task_id,
        model: task.data.model,
        prompt: task.prompt,
        options: task.options,
        references: task.references,
        can_regenerate: task.can_regenerate,
        created: task.data.created,
        images: task.data.images,
      })
    }
  }

  const imageTiles = items.flatMap((item) =>
    item.images.flatMap((image, index) => {
      const source = generatedImageSource(image)
      if (!source) return []
      return [
        {
          item,
          image,
          index,
          source,
          action: {
            task_id: item.task_id,
            index,
            model: item.model,
            prompt: item.prompt ?? '',
            url: source,
            outputFormat: image.output_format,
            options: item.options,
            references: item.references,
            canRegenerate: item.can_regenerate,
          },
        },
      ]
    })
  )
  const selectedTiles = imageTiles.filter(({ item, index }) =>
    selectedImageKeys.has(`${item.task_id}:${index}`)
  )
  const selectedActions = selectedTiles.map((tile) => tile.action)
  const totalTiles = imageTiles.length + previewCount
  const pageCapacity = props.compact ? 12 : layout.capacity
  const pageCount = Math.max(1, Math.ceil(totalTiles / pageCapacity))
  const currentPage = Math.min(page, pageCount - 1)
  const firstTileIndex = currentPage * pageCapacity
  const visibleImages = imageTiles.slice(
    Math.max(0, firstTileIndex - previewCount),
    Math.max(0, firstTileIndex + pageCapacity - previewCount)
  )
  const showPreview = taskIsActive && currentPage === 0

  const downloadImage = async (
    item: ImageGenerationHistoryItem,
    index: number,
    image: GeneratedImage
  ) => {
    if (downloadingImage !== null) return
    setDownloadingImage(`${item.task_id}:${index}`)
    try {
      const blob = await getImageGenerationDownload(item.task_id, index)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `generated-image-${item.task_id}-${index + 1}.${image.output_format || 'png'}`
      document.body.append(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(url)
    } catch (error) {
      handleServerError(error)
    } finally {
      setDownloadingImage(null)
    }
  }

  const downloadSelectedImages = async () => {
    if (downloadingImage !== null || selectedTiles.length === 0) return
    if (selectedTiles.length === 1) {
      const { item, index, image } = selectedTiles[0]
      await downloadImage(item, index, image)
      return
    }
    setDownloadingImage('archive')
    try {
      const blob = await createImageDownloadArchive(
        selectedTiles.map(({ item, index, image }) => ({
          taskId: item.task_id,
          index,
          outputFormat: image.output_format,
        })),
        getImageGenerationDownload
      )
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = 'generated-images.zip'
      document.body.append(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(url)
    } catch (error) {
      handleServerError(error)
    } finally {
      setDownloadingImage(null)
    }
  }

  useLayoutEffect(() => {
    if (props.compact) return
    const grid = gridRef.current
    if (!grid) return

    const updateLayout = () => {
      const bounds = grid.getBoundingClientRect()
      if (bounds.width <= 0 || bounds.height <= 0) return
      const next = calculateImageGridLayout(
        bounds.width,
        bounds.height,
        tileSize
      )
      setLayout((previous) =>
        previous.columns === next.columns && previous.capacity === next.capacity
          ? previous
          : next
      )
    }
    updateLayout()
    if (typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(updateLayout)
    observer.observe(grid)
    return () => observer.disconnect()
  }, [props.compact, tileSize, totalTiles])

  useEffect(() => setPage(0), [props.taskKey, props.task?.task_id])
  useEffect(
    () => setPage((previous) => Math.min(previous, pageCount - 1)),
    [pageCount]
  )

  if (props.isError && items.length === 0) {
    return (
      <ErrorState
        title={t('Could not load the generation task')}
        description={t('Check the connection and try again.')}
        onRetry={props.onRetry}
        className='h-full min-h-[320px] sm:min-h-[420px]'
      />
    )
  }

  if (props.isHistoryError && items.length === 0 && !taskIsActive) {
    return (
      <ErrorState
        title={t('Could not load image history')}
        description={t('Check the connection and try again.')}
        onRetry={props.onRetry}
        className='h-full min-h-[320px] sm:min-h-[420px]'
      />
    )
  }

  if (props.isHistoryLoading && items.length === 0 && !taskIsActive) {
    return (
      <div className='flex min-h-0 flex-1 items-center justify-center p-3 sm:p-5'>
        <Skeleton className='aspect-square w-full max-w-72 rounded-xl' />
      </div>
    )
  }

  if (!taskIsActive && items.length === 0 && !failedTask) {
    return (
      <div className='flex min-h-0 flex-1 items-center justify-center'>
        <EmptyState
          icon={WandSparkles}
          title={t('Your generated images will appear here')}
          description={t(
            'Choose a model, describe your idea, and start generating.'
          )}
          className='min-h-0 border-0'
        />
      </div>
    )
  }

  const imageCount = imageTiles.length

  return (
    <div className='flex min-h-0 min-w-0 flex-1 flex-col gap-4 p-3 sm:gap-5 sm:p-5 xl:h-full'>
      <div className='flex shrink-0 flex-wrap items-center justify-between gap-3'>
        <p className='min-w-0 text-base leading-snug font-medium'>
          {t('Generated images')}
        </p>
        <div className='flex items-center gap-2'>
          {taskIsActive ? (
            <Badge variant='secondary' className='gap-1.5'>
              <LoaderCircle
                className='size-3.5 animate-spin'
                aria-hidden='true'
              />
              {t('Generating your image')}
            </Badge>
          ) : null}
          <Badge
            variant='secondary'
            aria-label={t('{{count}} saved images', { count: imageCount })}
            className='tabular-nums'
          >
            {imageCount}
          </Badge>
        </div>
      </div>

      {failedTask ? (
        <div
          role='alert'
          className='border-destructive/30 bg-destructive/5 rounded-lg border p-3 text-sm'
        >
          <p className='font-medium'>{t('Image generation failed')}</p>
          <p className='text-muted-foreground mt-1'>
            {failedTask.error ||
              t('The image provider did not complete this request.')}
          </p>
        </div>
      ) : null}

      <div className='flex shrink-0 items-center justify-end border-b pb-3'>
        <div className='flex w-full max-w-48 items-center gap-3'>
          <ImageIcon
            className='text-muted-foreground size-4 shrink-0'
            aria-hidden='true'
          />
          <Slider
            id='image-grid-size'
            aria-label={t('Image grid size')}
            min={120}
            max={360}
            step={10}
            value={[tileSize]}
            onValueChange={(value) => {
              const next = Array.isArray(value) ? value[0] : value
              setTileSize(next)
            }}
          />
          <span className='text-muted-foreground min-w-7 text-right text-xs'>
            {Math.round(tileSize / 10)}
          </span>
        </div>
      </div>

      <div
        ref={gridRef}
        className='grid min-w-0 content-start gap-3 overflow-visible xl:min-h-0 xl:flex-1 xl:overflow-hidden'
        style={{
          gridTemplateColumns: props.compact
            ? `repeat(auto-fill, minmax(min(100%, ${tileSize}px), 1fr))`
            : `repeat(${layout.columns}, minmax(0, 1fr))`,
        }}
      >
        {showPreview
          ? Array.from(
              { length: Math.min(previewCount, pageCapacity) },
              (_, index) => (
                <Skeleton
                  key={`preview-${index}`}
                  className='aspect-square w-full rounded-xl'
                  aria-label={t('Generating your image')}
                />
              )
            )
          : null}
        {visibleImages.map(({ item, image, index, source, action }) => {
          const key = `${item.task_id}:${index}`
          const selected = selectedImageKeys.has(key)
          return (
            <figure
              key={key}
              className={cn(
                'group bg-muted/30 relative aspect-square min-w-0 overflow-hidden rounded-xl border',
                selected && 'ring-primary ring-2'
              )}
            >
              <img
                src={source}
                alt={t('Generated image {{number}}', { number: index + 1 })}
                loading='lazy'
                className='size-full object-contain'
              />
              <Checkbox
                checked={selected}
                onCheckedChange={(checked) =>
                  setSelectedImageKeys((previous) => {
                    const next = new Set(previous)
                    if (checked) next.add(key)
                    else next.delete(key)
                    return next
                  })
                }
                aria-label={t('Select generated image {{number}}', {
                  number: index + 1,
                })}
                className='bg-background/90 absolute top-2 left-2 z-10 size-6 rounded-full border-2 opacity-0 shadow-sm transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 data-checked:opacity-100 [@media(hover:none)]:opacity-100'
              />
              {props.onRegenerate || props.onEdit || props.onDelete ? (
                <DropdownMenu>
                  <DropdownMenuTrigger
                    render={
                      <Button
                        type='button'
                        size='icon-sm'
                        variant='secondary'
                        className='bg-background/90 absolute top-2 right-2 z-10 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100'
                        aria-label={t('Image {{index}} actions', {
                          index: index + 1,
                        })}
                      />
                    }
                  >
                    <Ellipsis aria-hidden='true' />
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align='end' className='w-40'>
                    {props.onRegenerate ? (
                      <DropdownMenuItem
                        disabled={
                          (props.actionsDisabled ?? props.isLoading) ||
                          action.canRegenerate === false
                        }
                        onClick={() => props.onRegenerate?.([action])}
                      >
                        <RotateCw aria-hidden='true' />
                        {action.canRegenerate === false
                          ? t('Original generation settings unavailable')
                          : t('Regenerate')}
                      </DropdownMenuItem>
                    ) : null}
                    {props.onEdit ? (
                      <DropdownMenuItem onClick={() => props.onEdit?.(action)}>
                        <Pencil aria-hidden='true' />
                        {t('Edit')}
                      </DropdownMenuItem>
                    ) : null}
                    {props.onDelete ? (
                      <DropdownMenuItem
                        variant='destructive'
                        onClick={() =>
                          setPendingDelete([
                            { task_id: action.task_id, index: action.index },
                          ])
                        }
                      >
                        <Trash2 aria-hidden='true' />
                        {t('Delete')}
                      </DropdownMenuItem>
                    ) : null}
                  </DropdownMenuContent>
                </DropdownMenu>
              ) : null}
              <figcaption className='absolute right-2 bottom-2 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100'>
                <Button
                  size='icon-sm'
                  variant='secondary'
                  type='button'
                  aria-label={t('Download')}
                  aria-busy={downloadingImage === key}
                  disabled={downloadingImage !== null}
                  onClick={() => void downloadImage(item, index, image)}
                >
                  {downloadingImage === key ? (
                    <LoaderCircle className='animate-spin' aria-hidden='true' />
                  ) : (
                    <Download aria-hidden='true' />
                  )}
                </Button>
              </figcaption>
            </figure>
          )
        })}
      </div>

      <div
        className={cn(
          'flex min-h-11 shrink-0 items-center justify-center',
          selectedTiles.length === 0 && 'hidden xl:flex'
        )}
      >
        {selectedTiles.length > 0 ? (
          <div className='bg-background/95 flex items-center gap-1 rounded-full border p-1 shadow-lg'>
            <Button
              type='button'
              size='sm'
              className='rounded-full'
              disabled={downloadingImage !== null}
              onClick={() => void downloadSelectedImages()}
            >
              {downloadingImage === 'archive' ? (
                <LoaderCircle className='animate-spin' aria-hidden='true' />
              ) : (
                <Download aria-hidden='true' />
              )}
              {t('Download')}{' '}
              <span className='tabular-nums'>{selectedTiles.length}</span>
            </Button>
            {props.onRegenerate || props.onDelete ? (
              <DropdownMenu>
                <DropdownMenuTrigger
                  render={
                    <Button
                      type='button'
                      size='icon-sm'
                      variant='ghost'
                      className='rounded-full'
                      aria-label={t('Selected image actions')}
                    />
                  }
                >
                  <Ellipsis aria-hidden='true' />
                </DropdownMenuTrigger>
                <DropdownMenuContent align='end' className='w-44'>
                  {props.onRegenerate ? (
                    <DropdownMenuItem
                      disabled={
                        (props.actionsDisabled ?? props.isLoading) ||
                        selectedActions.some(
                          (image) => image.canRegenerate === false
                        )
                      }
                      onClick={() => props.onRegenerate?.(selectedActions)}
                    >
                      <RotateCw aria-hidden='true' />
                      {selectedActions.some(
                        (image) => image.canRegenerate === false
                      )
                        ? t('Original generation settings unavailable')
                        : t('Regenerate selected')}
                    </DropdownMenuItem>
                  ) : null}
                  {props.onDelete ? (
                    <DropdownMenuItem
                      variant='destructive'
                      onClick={() =>
                        setPendingDelete(
                          selectedActions.map(({ task_id, index }) => ({
                            task_id,
                            index,
                          }))
                        )
                      }
                    >
                      <Trash2 aria-hidden='true' />
                      {t('Delete selected')}
                    </DropdownMenuItem>
                  ) : null}
                </DropdownMenuContent>
              </DropdownMenu>
            ) : null}
            <Button
              type='button'
              size='icon-sm'
              variant='ghost'
              className='rounded-full'
              aria-label={t('Clear selection')}
              onClick={() => setSelectedImageKeys(new Set())}
            >
              <X aria-hidden='true' />
            </Button>
          </div>
        ) : null}
      </div>

      {pageCount > 1 || props.hasMore ? (
        <div className='flex shrink-0 items-center justify-between gap-2 border-t pt-3'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setPage(currentPage - 1)}
            disabled={currentPage === 0}
          >
            {t('Previous')}
          </Button>
          <span className='text-muted-foreground text-xs'>
            {props.hasMore
              ? `${t('Page')} ${currentPage + 1}`
              : t('Page {{current}} of {{total}}', {
                  current: currentPage + 1,
                  total: pageCount,
                })}
          </span>
          <Button
            variant='outline'
            size='sm'
            onClick={async () => {
              if (currentPage + 1 < pageCount) {
                setPage(currentPage + 1)
                return
              }
              if (props.hasMore && (await props.onLoadMore())) {
                setPage(currentPage + 1)
              }
            }}
            disabled={
              props.isLoadingMore ||
              (currentPage + 1 >= pageCount && !props.hasMore)
            }
          >
            {props.isLoadingMore ? t('Loading...') : t('Next')}
          </Button>
        </div>
      ) : null}
      <ConfirmDialog
        open={pendingDelete.length > 0}
        onOpenChange={(open) => {
          if (!open && !isDeleting) setPendingDelete([])
        }}
        title={t('Delete selected images')}
        desc={t('Remove {{count}} images from your saved gallery?', {
          count: pendingDelete.length,
        })}
        confirmText={t('Delete')}
        destructive
        isLoading={isDeleting}
        handleConfirm={() => {
          if (!props.onDelete || pendingDelete.length === 0) return
          setIsDeleting(true)
          void props
            .onDelete(pendingDelete)
            .then(() => {
              setSelectedImageKeys(new Set())
              setPendingDelete([])
            })
            .catch(handleServerError)
            .finally(() => setIsDeleting(false))
        }}
      />
    </div>
  )
}
