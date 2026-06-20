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
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Checkbox } from '@/components/ui/checkbox'
import { createAigcElement } from '../api'
import { ELEMENT_TAGS, REFERENCE_TYPE, SUCCESS_MESSAGES } from '../constants'
import type { CreateAigcElementPayload } from '../types'
import { useAigcElements } from './aigc-elements-provider'
import { ImageUploadButton } from './image-upload-button'

type AigcElementsMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

// Newline-separated URLs -> trimmed, non-empty list.
function splitUrls(raw: string): string[] {
  return raw
    .split(/[\n,]/)
    .map((s) => s.trim())
    .filter(Boolean)
}

export function AigcElementsMutateDrawer({
  open,
  onOpenChange,
}: AigcElementsMutateDrawerProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useAigcElements()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const formSchema = z.object({
    channel_id: z.string().optional(),
    name: z
      .string()
      .min(1, t('Name is required'))
      .max(20, t('Name cannot exceed 20 characters')),
    description: z
      .string()
      .min(1, t('Description is required'))
      .max(100, t('Description cannot exceed 100 characters')),
    reference_type: z.enum([REFERENCE_TYPE.IMAGE, REFERENCE_TYPE.VIDEO]),
    frontal_image: z.string().optional(),
    refer_images: z.string().optional(),
    video_list: z.string().optional(),
    element_voice_id: z.string().optional(),
    tag_ids: z.array(z.string()).optional(),
  })
  type FormValues = z.infer<typeof formSchema>

  const defaultValues: FormValues = {
    channel_id: '',
    name: '',
    description: '',
    reference_type: REFERENCE_TYPE.IMAGE,
    frontal_image: '',
    refer_images: '',
    video_list: '',
    element_voice_id: '',
    tag_ids: [],
  }

  const form = useForm<FormValues>({
    resolver: zodResolver(formSchema),
    defaultValues,
  })

  useEffect(() => {
    if (open) {
      form.reset(defaultValues)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  const referenceType = form.watch('reference_type')

  const onSubmit = async (data: FormValues) => {
    const isImage = data.reference_type === REFERENCE_TYPE.IMAGE
    const referImages = splitUrls(data.refer_images || '')
    const videoList = splitUrls(data.video_list || '')

    if (isImage) {
      if (!data.frontal_image?.trim()) {
        form.setError('frontal_image', {
          message: t('A frontal reference image is required'),
        })
        return
      }
      if (referImages.length < 1 || referImages.length > 3) {
        form.setError('refer_images', {
          message: t('Provide 1 to 3 additional reference images'),
        })
        return
      }
    } else if (videoList.length < 1) {
      form.setError('video_list', {
        message: t('A reference video URL is required'),
      })
      return
    }

    const payload: CreateAigcElementPayload = {
      name: data.name.trim(),
      description: data.description.trim(),
      reference_type: data.reference_type,
      element_voice_id: data.element_voice_id?.trim() || undefined,
      tag_ids: data.tag_ids && data.tag_ids.length > 0 ? data.tag_ids : undefined,
    }
    const channelId = Number.parseInt(data.channel_id || '', 10)
    if (!Number.isNaN(channelId) && channelId > 0) {
      payload.channel_id = channelId
    }
    if (isImage) {
      payload.frontal_image = data.frontal_image?.trim()
      payload.refer_images = referImages
    } else {
      payload.video_list = videoList
    }

    setIsSubmitting(true)
    try {
      const result = await createAigcElement(payload)
      if (result.success) {
        toast.success(t(SUCCESS_MESSAGES.ELEMENT_CREATED))
        onOpenChange(false)
        triggerRefresh()
      }
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) {form.reset()}
      }}
    >
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[600px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Create Subject')}</SheetTitle>
          <SheetDescription>
            {t(
              'Create a reusable subject (character/prop) on Tencent VCLM. It is owned by your account and can be referenced when generating videos.'
            )}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='aigc-element-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={t('Enter a name')} />
                    </FormControl>
                    <FormDescription>
                      {t('Subject name (max 20 characters)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='description'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Description')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        rows={3}
                        placeholder={t('Describe the subject')}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Subject description (max 100 characters)')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='reference_type'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Reference Type')}</FormLabel>
                    <FormControl>
                      <NativeSelect {...field}>
                        <NativeSelectOption value={REFERENCE_TYPE.IMAGE}>
                          {t('Image reference (image_refer)')}
                        </NativeSelectOption>
                        <NativeSelectOption value={REFERENCE_TYPE.VIDEO}>
                          {t('Video reference (video_refer)')}
                        </NativeSelectOption>
                      </NativeSelect>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {referenceType === REFERENCE_TYPE.IMAGE ? (
                <>
                  <FormField
                    control={form.control}
                    name='frontal_image'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Frontal Image')}</FormLabel>
                        <FormControl>
                          <Input
                            {...field}
                            placeholder='https://example.com/front.jpg'
                          />
                        </FormControl>
                        <div className='flex items-center gap-2'>
                          <ImageUploadButton
                            label={t('Upload Frontal Image')}
                            onUploaded={(url) =>
                              form.setValue('frontal_image', url, {
                                shouldValidate: true,
                              })
                            }
                          />
                          {field.value ? (
                            <img
                              src={field.value}
                              alt={t('Frontal preview')}
                              className='border-border h-10 w-10 rounded-md border object-cover'
                            />
                          ) : null}
                        </div>
                        <FormDescription>
                          {t('One frontal reference image, jpg/jpeg/png, max 10MB (upload a local file or paste a public URL)')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='refer_images'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Reference Images')}</FormLabel>
                        <FormControl>
                          <Textarea
                            {...field}
                            rows={3}
                            placeholder={'https://example.com/1.jpg\nhttps://example.com/2.jpg'}
                          />
                        </FormControl>
                        <div className='flex items-center gap-2'>
                          <ImageUploadButton
                            multiple
                            label={t('Upload Reference Images')}
                            onUploaded={(url) => {
                              const current = (
                                form.getValues('refer_images') || ''
                              ).trim()
                              form.setValue(
                                'refer_images',
                                current ? `${current}\n${url}` : url,
                                { shouldValidate: true }
                              )
                            }}
                          />
                        </div>
                        <FormDescription>
                          {t('1 to 3 additional images, jpg/jpeg/png, max 10MB each (upload local files or paste one URL per line)')}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </>
              ) : (
                <FormField
                  control={form.control}
                  name='video_list'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Reference Video URL')}</FormLabel>
                      <FormControl>
                        <Textarea
                          {...field}
                          rows={2}
                          placeholder='https://example.com/video.mp4'
                        />
                      </FormControl>
                      <div className='bg-muted/50 text-muted-foreground rounded-md p-2 text-xs leading-relaxed'>
                        <div className='text-foreground mb-1 font-medium'>
                          {t('Video requirements')}
                        </div>
                        <ul className='list-disc space-y-0.5 pl-4'>
                          <li>{t('Format: MP4 or MOV, max 200MB, one video only')}</li>
                          <li>{t('Duration 3-8s, 1080P, aspect ratio 16:9 or 9:16')}</li>
                          <li>{t('Only realistic human figures are supported')}</li>
                          <li>{t('Video subjects work only on kling-video-o3 and later models')}</li>
                          <li>{t('A video with human voice triggers voice customization')}</li>
                        </ul>
                      </div>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}

              <FormField
                control={form.control}
                name='element_voice_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Voice ID (optional)')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={t('Bind an existing voice library ID')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='tag_ids'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Tags (optional)')}</FormLabel>
                    <div className='grid grid-cols-2 gap-2 sm:grid-cols-4'>
                      {ELEMENT_TAGS.map((tag) => {
                        const checked = (field.value || []).includes(tag.id)
                        return (
                          <label
                            key={tag.id}
                            className='flex cursor-pointer items-center gap-2 text-sm'
                          >
                            <Checkbox
                              checked={checked}
                              onCheckedChange={(v) => {
                                const cur = field.value || []
                                field.onChange(
                                  v
                                    ? [...cur, tag.id]
                                    : cur.filter((x) => x !== tag.id)
                                )
                              }}
                            />
                            {t(tag.labelKey)}
                          </label>
                        )
                      })}
                    </div>
                    <FormDescription>
                      {t('Categorize the subject; helps organize and filter subjects')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='channel_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Channel ID (optional)')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        placeholder={t('Leave empty to auto-select a TencentVideo channel')}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Specify which TencentVideo channel to use; auto-selected when empty')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Close')}
          </SheetClose>
          <Button
            form='aigc-element-form'
            type='submit'
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Saving...') : t('Create')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
