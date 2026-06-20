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
import { useRef, useState } from 'react'
import { Upload, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { uploadAigcElementImage } from '../api'

type ImageUploadButtonProps = {
  // Called once per successfully uploaded file with its public URL.
  onUploaded: (url: string, info: { resized: boolean; sizeHuman: string }) => void
  // Allow selecting several files at once (for the reference-images field).
  multiple?: boolean
  label?: string
}

// ImageUploadButton lets users pick local image files; each is sent to the
// backend which auto-resizes to <=10MB, stores it, and returns a public URL.
// The user never has to host the image themselves.
export function ImageUploadButton({
  onUploaded,
  multiple = false,
  label,
}: ImageUploadButtonProps) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement>(null)
  const [isUploading, setIsUploading] = useState(false)

  const handleFiles = async (files: FileList | null) => {
    if (!files || files.length === 0) {return}
    setIsUploading(true)
    try {
      for (const file of files) {
        const result = await uploadAigcElementImage(file)
        if (result.success && result.data?.url) {
          onUploaded(result.data.url, {
            resized: result.data.resized,
            sizeHuman: result.data.size_human,
          })
          if (result.data.resized) {
            toast.success(
              t('{{name}} was resized to {{size}} and uploaded', {
                name: file.name,
                size: result.data.size_human,
              })
            )
          } else {
            toast.success(t('{{name}} uploaded', { name: file.name }))
          }
        }
      }
    } finally {
      setIsUploading(false)
      if (inputRef.current) {inputRef.current.value = ''}
    }
  }

  return (
    <>
      <input
        ref={inputRef}
        type='file'
        accept='image/png,image/jpeg,image/jpg'
        multiple={multiple}
        className='hidden'
        onChange={(e) => handleFiles(e.target.files)}
      />
      <Button
        type='button'
        variant='outline'
        size='sm'
        disabled={isUploading}
        onClick={() => inputRef.current?.click()}
      >
        {isUploading ? (
          <Loader2 className='h-4 w-4 animate-spin' />
        ) : (
          <Upload className='h-4 w-4' />
        )}
        {label || t('Upload Image')}
      </Button>
    </>
  )
}
