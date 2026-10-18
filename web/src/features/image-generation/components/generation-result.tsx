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

import { Download, ImageIcon, LoaderCircle, WandSparkles } from 'lucide-react'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Slider } from '@/components/ui/slider'
import { handleServerError } from '@/lib/handle-server-error'

import { getImageGenerationDownload } from '../api'
import {
  calculateImageGridLayout,
  generatedImageSource,
  isTerminalTaskStatus,
} from '../lib/image-generation'
import type {
  GeneratedImage,
  ImageGenerationHistoryItem,
  ImageGenerationTask,
} from '../types'

type GenerationResultProps = {
  compact?: boolean
  history: ImageGenerationHistoryItem[]
  hasMore: boolean
  isError: boolean
  isHistoryError: boolean
  isHistoryLoading: boolean
  isLoading: boolean
  isLoadingMore: boolean
  onLoadMore: () => Promise<boolean>
  onRetry: () => void
  task?: ImageGenerationTask
}

export function GenerationResult(props: GenerationResultProps) {
  const { t } = useTranslation()
  const [tileSize, setTileSize] = useState(props.compact ? 160 : 220)
  const [page, setPage] = useState(0)
  const [downloadingImage, setDownloadingImage] = useState<string | null>(null)
  const [layout, setLayout] = useState({ columns: 2, capacity: 6 })
  const gridRef = useRef<HTMLDivElement>(null)
  const taskIsActive =
    props.isLoading ||
    (props.task !== undefined && !isTerminalTaskStatus(props.task.status))
  const items = [...props.history]
  if (
    props.task?.status === 'SUCCESS' &&
    props.task.data?.images?.length &&
    !items.some((item) => item.task_id === props.task?.task_id)
  ) {
    items.unshift({
      task_id: props.task.task_id,
      model: props.task.data.model,
      created: props.task.data.created,
      images: props.task.data.images,
    })
  }

  const imageTiles = items.flatMap((item) =>
    item.images.flatMap((image, index) => {
      const source = generatedImageSource(image)
      if (!source) return []
      return [{ item, image, index, source }]
    })
  )
  const totalTiles = imageTiles.length + (taskIsActive ? 1 : 0)
  const pageCapacity = props.compact ? 12 : layout.capacity
  const pageCount = Math.max(1, Math.ceil(totalTiles / pageCapacity))
  const currentPage = Math.min(page, pageCount - 1)
  const firstTileIndex = currentPage * pageCapacity
  const visibleImages = imageTiles.slice(
    Math.max(0, firstTileIndex - (taskIsActive ? 1 : 0)),
    Math.max(0, firstTileIndex + pageCapacity - (taskIsActive ? 1 : 0))
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

  useEffect(() => setPage(0), [props.task?.task_id])
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

  if (!taskIsActive && items.length === 0 && props.task?.status !== 'FAILURE') {
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
        <div className='min-w-0'>
          <p className='font-medium'>{t('Generated images')}</p>
          <p className='text-muted-foreground text-xs'>
            {t('{{count}} saved images', { count: imageCount })}
          </p>
        </div>
        {taskIsActive ? (
          <Badge variant='secondary' className='gap-1.5'>
            <LoaderCircle
              className='size-3.5 animate-spin'
              aria-hidden='true'
            />
            {t('Generating your image')}
          </Badge>
        ) : null}
      </div>

      {props.task?.status === 'FAILURE' ? (
        <div
          role='alert'
          className='border-destructive/30 bg-destructive/5 rounded-lg border p-3 text-sm'
        >
          <p className='font-medium'>{t('Image generation failed')}</p>
          <p className='text-muted-foreground mt-1'>
            {props.task.error ||
              t('The image provider did not complete this request.')}
          </p>
        </div>
      ) : null}

      <div className='flex shrink-0 flex-wrap items-center justify-between gap-3 border-b pb-3'>
        <Label htmlFor='image-grid-size'>{t('Image grid size')}</Label>
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
        {showPreview ? (
          <Skeleton
            className='aspect-square w-full rounded-xl'
            aria-label={t('Generating your image')}
          />
        ) : null}
        {visibleImages.map(({ item, image, index, source }) => (
          <figure
            key={`${item.task_id}-${source}`}
            className='group bg-muted/30 relative aspect-square min-w-0 overflow-hidden rounded-xl border'
          >
            <img
              src={source}
              alt={t('Generated image {{number}}', { number: index + 1 })}
              loading='lazy'
              className='size-full object-contain'
            />
            <figcaption className='absolute right-2 bottom-2 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100'>
              <Button
                size='icon-sm'
                variant='secondary'
                type='button'
                aria-label={t('Download')}
                aria-busy={downloadingImage === `${item.task_id}:${index}`}
                disabled={downloadingImage !== null}
                onClick={() => void downloadImage(item, index, image)}
              >
                {downloadingImage === `${item.task_id}:${index}` ? (
                  <LoaderCircle className='animate-spin' aria-hidden='true' />
                ) : (
                  <Download aria-hidden='true' />
                )}
              </Button>
            </figcaption>
          </figure>
        ))}
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
    </div>
  )
}
