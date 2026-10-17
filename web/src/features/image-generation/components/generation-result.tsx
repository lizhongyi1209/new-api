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
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

import {
  generatedImageSource,
  isTerminalTaskStatus,
} from '../lib/image-generation'
import type { ImageGenerationTask } from '../types'

type GenerationResultProps = {
  isError: boolean
  isLoading: boolean
  onRetry: () => void
  task?: ImageGenerationTask
}

export function GenerationResult(props: GenerationResultProps) {
  const { t } = useTranslation()

  if (!props.task && !props.isLoading && !props.isError) {
    return (
      <EmptyState
        icon={WandSparkles}
        title={t('Your generated images will appear here')}
        description={t(
          'Choose a model, describe your idea, and start generating.'
        )}
        className='h-full min-h-[320px] border-0 sm:min-h-[420px] xl:min-h-[560px]'
      />
    )
  }

  if (props.isError) {
    return (
      <ErrorState
        title={t('Could not load the generation task')}
        description={t('Check the connection and try again.')}
        onRetry={props.onRetry}
        className='h-full min-h-[320px] sm:min-h-[420px] xl:min-h-[560px]'
      />
    )
  }

  if (!props.task || !isTerminalTaskStatus(props.task.status)) {
    return (
      <div className='flex h-full min-h-[320px] flex-col p-4 sm:min-h-[420px] sm:p-5 xl:min-h-[560px]'>
        <div className='mb-5 flex items-center justify-between gap-3'>
          <div className='flex items-center gap-2'>
            <LoaderCircle
              className='text-primary size-4 animate-spin'
              aria-hidden='true'
            />
            <p className='text-sm font-medium'>{t('Generating your image')}</p>
          </div>
          <Badge variant='secondary'>
            {props.task?.progress || t('Queued')}
          </Badge>
        </div>
        <div className='grid flex-1 gap-4 lg:grid-cols-2'>
          <Skeleton className='min-h-56 rounded-xl sm:min-h-72' />
          <Skeleton className='hidden min-h-56 rounded-xl sm:min-h-72 lg:block' />
        </div>
      </div>
    )
  }

  if (props.task.status === 'FAILURE') {
    return (
      <ErrorState
        title={t('Image generation failed')}
        description={
          props.task.error ||
          t('The image provider did not complete this request.')
        }
        className='h-full min-h-[320px] sm:min-h-[420px] xl:min-h-[560px]'
      />
    )
  }

  const images = props.task.data?.images ?? []
  if (images.length === 0) {
    return (
      <EmptyState
        icon={ImageIcon}
        title={t('No images were returned')}
        description={t('The task completed without a usable image result.')}
        className='h-full min-h-[320px] border-0 sm:min-h-[420px] xl:min-h-[560px]'
      />
    )
  }

  return (
    <div className='space-y-4 p-3 sm:p-5'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div>
          <p className='font-medium'>{t('Generated images')}</p>
          <p className='text-muted-foreground text-xs'>
            {props.task.data?.model}
          </p>
        </div>
        <Badge variant='secondary'>{t('Completed')}</Badge>
      </div>
      <div className='grid gap-3 sm:gap-4 lg:grid-cols-2'>
        {images.map((image, index) => {
          const source = generatedImageSource(image)
          return (
            <figure
              key={source}
              className='group bg-muted/30 relative overflow-hidden rounded-xl border'
            >
              <img
                src={source}
                alt={t('Generated image {{number}}', { number: index + 1 })}
                className='aspect-square size-full object-contain'
              />
              <figcaption className='absolute right-2 bottom-2 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100'>
                <Button
                  size='sm'
                  variant='secondary'
                  render={
                    <a
                      href={source}
                      download={`generated-image-${index + 1}.${image.output_format || 'png'}`}
                      target='_blank'
                      rel='noreferrer'
                    />
                  }
                >
                  <Download aria-hidden='true' />
                  {t('Download')}
                </Button>
              </figcaption>
            </figure>
          )
        })}
      </div>
    </div>
  )
}
