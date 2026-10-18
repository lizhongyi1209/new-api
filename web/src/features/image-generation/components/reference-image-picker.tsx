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
  ArrowLeft,
  ArrowRight,
  Ellipsis,
  Plus,
  Replace,
  Trash2,
} from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
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
  const replaceInputRef = useRef<HTMLInputElement | null>(null)
  const replaceImageIdRef = useRef<string | null>(null)
  const [dragActive, setDragActive] = useState(false)
  const [draggedImageId, setDraggedImageId] = useState<string | null>(null)
  const [dropTargetId, setDropTargetId] = useState<string | null>(null)

  const createReferenceImage = (file: File): ReferenceImage | null => {
    if (!ACCEPTED_REFERENCE_IMAGE_TYPES.has(file.type)) {
      toast.error(t('{{name}} is not a supported image.', { name: file.name }))
      return null
    }
    if (file.size > MAX_REFERENCE_IMAGE_BYTES) {
      toast.error(t('{{name}} exceeds the 10 MiB limit.', { name: file.name }))
      return null
    }
    const previewUrl = URL.createObjectURL(file)
    return { file, id: previewUrl, previewUrl }
  }

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
      const image = createReferenceImage(file)
      if (image) accepted.push(image)
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

  const replaceImage = (imageId: string, file: File) => {
    const index = props.images.findIndex((image) => image.id === imageId)
    if (index < 0) return
    const replacement = createReferenceImage(file)
    if (!replacement) return
    const nextImages = [...props.images]
    nextImages[index] = replacement
    props.onChange(nextImages)
    URL.revokeObjectURL(props.images[index].previewUrl)
  }

  const moveImage = (fromIndex: number, toIndex: number) => {
    if (
      fromIndex < 0 ||
      toIndex < 0 ||
      fromIndex >= props.images.length ||
      toIndex >= props.images.length ||
      fromIndex === toIndex
    ) {
      return
    }
    const nextImages = [...props.images]
    const [movedImage] = nextImages.splice(fromIndex, 1)
    nextImages.splice(toIndex, 0, movedImage)
    props.onChange(nextImages)
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
        role='region'
        aria-label={t('Reference images')}
        tabIndex={0}
        className='flex min-w-0 [scrollbar-width:thin] [scrollbar-color:var(--border)_transparent] flex-nowrap gap-2 overflow-x-auto overscroll-x-contain pb-2'
      >
        {props.images.length < MAX_REFERENCE_IMAGES ? (
          <Button
            type='button'
            variant='outline'
            className={cn(
              'bg-muted/20 text-muted-foreground hover:bg-muted/35 size-20 shrink-0 rounded-lg border-dashed p-0 transition-colors @sm:size-24',
              dragActive && 'border-primary bg-primary/5'
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
              if (!props.disabled) addFiles(event.dataTransfer.files)
            }}
            disabled={props.disabled}
            aria-label={t('Add reference images')}
          >
            <Plus className='size-6' aria-hidden='true' />
          </Button>
        ) : null}
        {props.images.map((image, index) => (
          <figure
            key={image.id}
            draggable={!props.disabled}
            onDragStart={(event) => {
              event.dataTransfer.effectAllowed = 'move'
              event.dataTransfer.setData('text/plain', image.id)
              setDraggedImageId(image.id)
            }}
            onDragEnd={() => {
              setDraggedImageId(null)
              setDropTargetId(null)
            }}
            onDragOver={(event) => {
              if (!props.disabled) {
                event.preventDefault()
                if (draggedImageId && draggedImageId !== image.id) {
                  setDropTargetId(image.id)
                }
              }
            }}
            onDragLeave={() => setDropTargetId(null)}
            onDrop={(event) => {
              event.preventDefault()
              if (props.disabled) return
              if (event.dataTransfer.files.length > 0) {
                replaceImage(image.id, event.dataTransfer.files[0])
              } else if (draggedImageId) {
                moveImage(
                  props.images.findIndex((item) => item.id === draggedImageId),
                  index
                )
              }
              setDraggedImageId(null)
              setDropTargetId(null)
            }}
            className={cn(
              'group bg-muted relative m-0 size-20 shrink-0 overflow-hidden rounded-lg border @sm:size-24',
              !props.disabled && 'cursor-grab active:cursor-grabbing',
              dropTargetId === image.id && 'ring-primary ring-2 ring-offset-2'
            )}
          >
            <img
              src={image.previewUrl}
              alt={t('Reference image {{index}}: {{name}}', {
                index: index + 1,
                name: image.file.name,
              })}
              draggable={false}
              className='size-full object-cover'
            />
            <span
              aria-hidden='true'
              className='absolute top-1 left-1 flex size-5 items-center justify-center rounded bg-black/65 text-[10px] font-medium text-white'
            >
              {index + 1}
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button
                    type='button'
                    variant='secondary'
                    size='icon-xs'
                    className='bg-background/90 absolute top-1 right-1'
                    aria-label={t('Image {{index}} actions', {
                      index: index + 1,
                    })}
                    disabled={props.disabled}
                  />
                }
              >
                <Ellipsis aria-hidden='true' />
              </DropdownMenuTrigger>
              <DropdownMenuContent align='end' className='w-40'>
                <DropdownMenuItem
                  onClick={() => {
                    replaceImageIdRef.current = image.id
                    replaceInputRef.current?.click()
                  }}
                >
                  <Replace aria-hidden='true' />
                  {t('Replace')}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={index === 0}
                  onClick={() => moveImage(index, index - 1)}
                >
                  <ArrowLeft aria-hidden='true' />
                  {t('Move left')}
                </DropdownMenuItem>
                <DropdownMenuItem
                  disabled={index === props.images.length - 1}
                  onClick={() => moveImage(index, index + 1)}
                >
                  <ArrowRight aria-hidden='true' />
                  {t('Move right')}
                </DropdownMenuItem>
                <DropdownMenuItem
                  variant='destructive'
                  onClick={() => removeImage(image)}
                >
                  <Trash2 aria-hidden='true' />
                  {t('Remove')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </figure>
        ))}
      </div>
      <p className='text-muted-foreground text-xs'>
        {t('PNG, JPEG or WebP, up to 10 MiB each')}
      </p>
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
      <input
        ref={replaceInputRef}
        className='sr-only'
        type='file'
        accept='image/png,image/jpeg,image/webp'
        disabled={props.disabled}
        aria-label={t('Replace reference image')}
        onChange={(event) => {
          const file = event.target.files?.[0]
          const imageId = replaceImageIdRef.current
          if (file && imageId) replaceImage(imageId, file)
          event.target.value = ''
          replaceImageIdRef.current = null
        }}
      />
    </div>
  )
}
