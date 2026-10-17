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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { Slider } from '@/components/ui/slider'

import {
  generatedImageSource,
  isTerminalTaskStatus,
} from '../lib/image-generation'
import type { ImageGenerationHistoryItem, ImageGenerationTask } from '../types'

type GenerationResultProps = {
  history: ImageGenerationHistoryItem[]
  hasMore: boolean
  isError: boolean
  isHistoryError: boolean
  isHistoryLoading: boolean
  isLoading: boolean
  isLoadingMore: boolean
  onLoadMore: () => void
  onRetry: () => void
  task?: ImageGenerationTask
}

export function GenerationResult(props: GenerationResultProps) {
  const { t } = useTranslation()
  const [tileSize, setTileSize] = useState(220)
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
      <div className='p-3 sm:p-5'>
        <Skeleton className='aspect-square w-full max-w-72 rounded-xl' />
      </div>
    )
  }

  if (!taskIsActive && items.length === 0 && props.task?.status !== 'FAILURE') {
    return (
      <EmptyState
        icon={WandSparkles}
        title={t('Your generated images will appear here')}
        description={t(
          'Choose a model, describe your idea, and start generating.'
        )}
        className='h-full min-h-[320px] border-0 sm:min-h-[420px]'
      />
    )
  }

  const imageCount = items.reduce(
    (count, item) => count + item.images.length,
    0
  )

  return (
    <div className='flex min-w-0 flex-col gap-4 p-3 sm:gap-5 sm:p-5'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
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

      <div className='flex flex-wrap items-center justify-between gap-3 border-b pb-3'>
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
        className='grid min-w-0 content-start gap-3 sm:gap-4'
        style={{
          gridTemplateColumns: `repeat(auto-fill, minmax(min(100%, ${tileSize}px), 1fr))`,
        }}
      >
        {taskIsActive ? (
          <Skeleton
            className='aspect-square w-full rounded-xl'
            aria-label={t('Generating your image')}
          />
        ) : null}
        {items.flatMap((item) =>
          item.images.map((image, index) => {
            const source = generatedImageSource(image)
            if (!source) return []
            return [
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
                <figcaption className='absolute right-2 bottom-2 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100'>
                  <Button
                    size='sm'
                    variant='secondary'
                    render={
                      <a
                        href={source}
                        download={`generated-image-${item.task_id}-${index + 1}.${image.output_format || 'png'}`}
                        target='_blank'
                        rel='noreferrer'
                      />
                    }
                  >
                    <Download aria-hidden='true' />
                    {t('Download')}
                  </Button>
                </figcaption>
              </figure>,
            ]
          })
        )}
      </div>

      {props.hasMore ? (
        <Button
          variant='outline'
          className='self-center'
          onClick={props.onLoadMore}
          disabled={props.isLoadingMore}
        >
          {props.isLoadingMore ? t('Loading...') : t('Load more images')}
        </Button>
      ) : null}
    </div>
  )
}
