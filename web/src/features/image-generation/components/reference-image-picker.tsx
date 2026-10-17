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

import { ImagePlus, X } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import {
  ACCEPTED_REFERENCE_IMAGE_TYPES,
  MAX_REFERENCE_IMAGE_BYTES,
  MAX_REFERENCE_IMAGES,
} from '../lib/image-generation'
import type { ReferenceImage } from '../types'

type ReferenceImagePickerProps = {
  disabled: boolean
  images: ReferenceImage[]
  onChange: (images: ReferenceImage[]) => void
}

export function ReferenceImagePicker(props: ReferenceImagePickerProps) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement | null>(null)
  const [dragActive, setDragActive] = useState(false)

  const addFiles = (files: FileList | File[]) => {
    const available = MAX_REFERENCE_IMAGES - props.images.length
    if (available <= 0) {
      toast.error(
        t('You can add up to {{count}} reference images.', {
          count: MAX_REFERENCE_IMAGES,
        })
      )
      return
    }

    const accepted: ReferenceImage[] = []
    for (const file of [...files].slice(0, available)) {
      if (!ACCEPTED_REFERENCE_IMAGE_TYPES.has(file.type)) {
        toast.error(
          t('{{name}} is not a supported image.', { name: file.name })
        )
        continue
      }
      if (file.size > MAX_REFERENCE_IMAGE_BYTES) {
        toast.error(
          t('{{name}} exceeds the 20 MiB limit.', { name: file.name })
        )
        continue
      }
      const previewUrl = URL.createObjectURL(file)
      accepted.push({
        file,
        id: previewUrl,
        previewUrl,
      })
    }

    if (accepted.length > 0) props.onChange([...props.images, ...accepted])
    if ([...files].length > available) {
      toast.error(
        t('You can add up to {{count}} reference images.', {
          count: MAX_REFERENCE_IMAGES,
        })
      )
    }
  }

  const removeImage = (image: ReferenceImage) => {
    URL.revokeObjectURL(image.previewUrl)
    props.onChange(props.images.filter((item) => item.id !== image.id))
  }

  return (
    <div className='space-y-2'>
      <div className='flex items-center justify-between gap-3'>
        <p className='text-sm font-medium'>{t('Reference images')}</p>
        <span className='text-muted-foreground text-xs'>
          {props.images.length}/{MAX_REFERENCE_IMAGES}
        </span>
      </div>
      <div
        className={cn(
          'border-border bg-muted/20 hover:bg-muted/35 flex min-h-24 cursor-pointer flex-col items-center justify-center rounded-xl border border-dashed px-3 py-4 text-center transition-colors sm:min-h-28 sm:px-4 sm:py-5',
          dragActive && 'border-primary bg-primary/5',
          props.disabled && 'pointer-events-none opacity-50'
        )}
        onClick={() => inputRef.current?.click()}
        onDragEnter={(event) => {
          event.preventDefault()
          setDragActive(true)
        }}
        onDragLeave={(event) => {
          event.preventDefault()
          setDragActive(false)
        }}
        onDragOver={(event) => event.preventDefault()}
        onDrop={(event) => {
          event.preventDefault()
          setDragActive(false)
          addFiles(event.dataTransfer.files)
        }}
        role='button'
        tabIndex={props.disabled ? -1 : 0}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault()
            inputRef.current?.click()
          }
        }}
        aria-label={t('Add reference images')}
      >
        <ImagePlus
          className='text-muted-foreground mb-2 size-5'
          aria-hidden='true'
        />
        <p className='text-sm font-medium'>
          {t('Drop images here or click to upload')}
        </p>
        <p className='text-muted-foreground mt-1 text-xs'>
          {t('PNG, JPEG or WebP, up to 20 MiB each')}
        </p>
      </div>
      <input
        ref={inputRef}
        className='sr-only'
        type='file'
        multiple
        accept='image/png,image/jpeg,image/webp'
        disabled={props.disabled}
        aria-label={t('Add reference images')}
        onChange={(event) => {
          if (event.target.files) addFiles(event.target.files)
          event.target.value = ''
        }}
      />

      {props.images.length > 0 ? (
        <div className='grid grid-cols-3 gap-2 sm:grid-cols-5'>
          {props.images.map((image) => (
            <div
              key={image.id}
              className='group bg-muted relative aspect-square overflow-hidden rounded-lg border'
            >
              <img
                src={image.previewUrl}
                alt={image.file.name}
                className='size-full object-cover'
              />
              <Button
                type='button'
                variant='destructive'
                size='icon-xs'
                className='absolute top-1 right-1 opacity-90'
                aria-label={t('Remove {{name}}', { name: image.file.name })}
                onClick={() => removeImage(image)}
                disabled={props.disabled}
              >
                <X aria-hidden='true' />
              </Button>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  )
}
