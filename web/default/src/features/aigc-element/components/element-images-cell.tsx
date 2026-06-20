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
import { ImageDialog } from '@/features/usage-logs/components/dialogs/image-dialog'
import type { AigcElement } from '../types'

// parseImageUrls returns the subject's reference image URLs: the frontal image
// first, then any additional reference images stored as a JSON array.
function parseImageUrls(element: AigcElement): string[] {
  const urls: string[] = []
  if (element.frontal_image) {
    urls.push(element.frontal_image)
  }
  if (element.refer_images) {
    try {
      const parsed = JSON.parse(element.refer_images)
      if (Array.isArray(parsed)) {
        for (const u of parsed) {
          if (typeof u === 'string' && u) {urls.push(u)}
        }
      }
    } catch {
      // ignore malformed JSON
    }
  }
  return urls
}

export function ElementImagesCell({ element }: { element: AigcElement }) {
  const { t } = useTranslation()
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  const urls = parseImageUrls(element)

  if (urls.length === 0) {
    return <span className='text-muted-foreground text-sm'>-</span>
  }

  return (
    <>
      <div className='flex items-center gap-1'>
        {urls.slice(0, 4).map((url) => (
          <button
            key={url}
            type='button'
            onClick={() => setPreviewUrl(url)}
            className='border-border h-10 w-10 shrink-0 overflow-hidden rounded-md border transition-opacity hover:opacity-80'
            title={t('Click to preview')}
          >
            <img
              src={url}
              alt={t('Reference image')}
              className='h-full w-full object-cover'
              loading='lazy'
            />
          </button>
        ))}
        {urls.length > 4 && (
          <span className='text-muted-foreground text-xs'>
            +{urls.length - 4}
          </span>
        )}
      </div>
      <ImageDialog
        imageUrl={previewUrl || ''}
        open={!!previewUrl}
        onOpenChange={(open) => !open && setPreviewUrl(null)}
      />
    </>
  )
}
