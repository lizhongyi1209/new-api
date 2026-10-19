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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'

interface ImageDialogProps {
  imageUrl: string
  taskId?: string
  open: boolean
  onOpenChange: (open: boolean) => void
  variant?: 'default' | 'gallery'
}

export function ImageDialog(props: ImageDialogProps) {
  const { t } = useTranslation()
  const [isLoading, setIsLoading] = useState(true)
  const [hasError, setHasError] = useState(false)
  const isGallery = props.variant === 'gallery'

  const handleOpenChange = (open: boolean) => {
    if (open) {
      setIsLoading(true)
      setHasError(false)
    }
    props.onOpenChange(open)
  }

  const imageContent = (
    <div className='py-4'>
      <div className='bg-muted/50 relative flex min-h-[300px] items-center justify-center rounded-lg border'>
        {(isLoading || hasError) && (
          <Skeleton className='absolute inset-0 h-full w-full rounded-lg' />
        )}
        <img
          src={props.imageUrl}
          alt={t('Generated image')}
          className={`w-full rounded-lg object-contain ${
            isGallery ? 'max-h-[calc(100dvh-12rem)]' : 'max-h-[550px]'
          } ${isLoading || hasError ? 'opacity-0' : 'opacity-100'}`}
          onLoad={() => {
            setIsLoading(false)
            setHasError(false)
          }}
          onError={() => {
            setIsLoading(false)
            setHasError(true)
          }}
        />
        {hasError && (
          <div className='absolute inset-0 flex items-center justify-center'>
            <p className='text-muted-foreground text-sm'>
              {t('Failed to load image')}
            </p>
          </div>
        )}
      </div>

      {!isGallery && (
        <div className='bg-muted mt-4 rounded-md p-3'>
          <p className='text-muted-foreground font-mono text-xs break-all'>
            {props.imageUrl}
          </p>
        </div>
      )}
    </div>
  )

  return (
    <Dialog
      open={props.open}
      onOpenChange={handleOpenChange}
      title={t('Image Preview')}
      description={
        props.taskId
          ? `${t('Task ID:')} ${props.taskId}`
          : t('View the generated image')
      }
      contentClassName={
        isGallery ? 'sm:max-w-[min(96vw,80rem)] sm:p-4' : 'sm:max-w-3xl'
      }
      contentHeight='auto'
      bodyClassName='space-y-4'
    >
      {isGallery ? (
        <div className='max-h-[calc(100dvh-8rem)] overflow-auto'>
          {imageContent}
        </div>
      ) : (
        <ScrollArea className='max-h-[600px]'>{imageContent}</ScrollArea>
      )}
    </Dialog>
  )
}
