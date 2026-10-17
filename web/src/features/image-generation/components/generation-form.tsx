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

import { zodResolver } from '@hookform/resolvers/zod'
import { LoaderCircle, Sparkles } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import {
  getImageModelDisplayName,
  getImageModelFamily,
} from '../lib/image-generation'
import { imageGenerationSchema } from '../lib/schema'
import type {
  GroupOption,
  ImageGenerationFormValue,
  ReferenceImage,
  SelectOption,
} from '../types'
import { GenerationOptions } from './generation-options'
import { ReferenceImagePicker } from './reference-image-picker'

type GenerationFormProps = {
  disabled: boolean
  groups: GroupOption[]
  isLoadingModels: boolean
  models: SelectOption[]
  onGroupChange: (group: string) => void
  onSubmit: (
    value: ImageGenerationFormValue,
    references: ReferenceImage[]
  ) => void
}

const defaultValues: ImageGenerationFormValue = {
  aspectRatio: '1:1',
  background: 'auto',
  group: '',
  model: '',
  moderation: 'auto',
  outputFormat: 'png',
  prompt: '',
  quality: 'auto',
  size: '1K',
  thinkingLevel: 'default',
}

export function GenerationForm(props: GenerationFormProps) {
  const { t } = useTranslation()
  const [references, setReferences] = useState<ReferenceImage[]>([])
  const previewUrlsRef = useRef(new Set<string>())
  const form = useForm<ImageGenerationFormValue>({
    defaultValues,
    resolver: zodResolver(imageGenerationSchema),
  })
  const selectedGroup = useWatch({ control: form.control, name: 'group' })
  const selectedModel = useWatch({ control: form.control, name: 'model' })
  const family = getImageModelFamily(selectedModel)
  const modelOptions = useMemo(() => {
    const tiers = {
      specialPrice: t('Special price'),
      premium: t('Premium'),
      enterprise: t('Enterprise'),
    }
    return props.models.map((model) => ({
      ...model,
      label: getImageModelDisplayName(model.value, tiers),
    }))
  }, [props.models, t])
  const bananaModels = modelOptions.filter(
    (model) => getImageModelFamily(model.value) === 'banana'
  )
  const gptImageModels = modelOptions.filter(
    (model) => getImageModelFamily(model.value) === 'gpt-image'
  )
  const groups = props.groups
  const onGroupChange = props.onGroupChange
  const models = props.models

  useEffect(() => {
    if (groups.length === 0) return
    if (groups.some((group) => group.value === selectedGroup)) return
    const nextGroup = groups[0].value
    form.setValue('group', nextGroup)
    onGroupChange(nextGroup)
  }, [form, groups, onGroupChange, selectedGroup])

  useEffect(() => {
    if (models.length === 0) {
      form.setValue('model', '')
      return
    }
    if (models.some((model) => model.value === selectedModel)) return
    const nextModel = models[0].value
    form.setValue('model', nextModel)
    const nextFamily = getImageModelFamily(nextModel)
    form.setValue('size', nextFamily === 'banana' ? '1K' : 'auto')
  }, [form, models, selectedModel])

  useEffect(() => {
    const previewUrls = previewUrlsRef.current
    return () => {
      for (const previewUrl of previewUrls) {
        URL.revokeObjectURL(previewUrl)
      }
    }
  }, [])

  const updateReferences = (nextReferences: ReferenceImage[]) => {
    for (const reference of nextReferences) {
      previewUrlsRef.current.add(reference.previewUrl)
    }
    setReferences(nextReferences)
  }

  const selectModel = (model: string) => {
    form.setValue('model', model, { shouldValidate: true })
    const nextFamily = getImageModelFamily(model)
    form.setValue('size', nextFamily === 'banana' ? '1K' : 'auto')
    form.setValue('outputFormat', 'png')
    form.setValue('background', 'auto')
    form.setValue('moderation', 'auto')
    form.setValue('quality', 'auto')
    form.setValue('thinkingLevel', 'default')
  }

  return (
    <Form {...form}>
      <form
        className='space-y-4 sm:space-y-5'
        onSubmit={form.handleSubmit((value) =>
          props.onSubmit(value, references)
        )}
      >
        <ReferenceImagePicker
          disabled={props.disabled}
          images={references}
          onChange={updateReferences}
        />

        <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-1'>
          <FormField
            control={form.control}
            name='group'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Group')}</FormLabel>
                <Select
                  items={props.groups}
                  value={field.value}
                  onValueChange={(value) => {
                    if (!value) return
                    field.onChange(value)
                    form.setValue('model', '')
                    props.onGroupChange(value)
                  }}
                  disabled={props.disabled || props.groups.length === 0}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue placeholder={t('Select a group')} />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent
                    alignItemWithTrigger={false}
                    className='max-h-[min(15rem,var(--available-height))] overscroll-contain'
                  >
                    <SelectGroup>
                      {props.groups.map((group) => (
                        <SelectItem key={group.value} value={group.value}>
                          {group.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='model'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Model')}</FormLabel>
                <Select
                  items={modelOptions}
                  value={field.value}
                  onValueChange={(value) => value && selectModel(value)}
                  disabled={
                    props.disabled ||
                    props.isLoadingModels ||
                    props.models.length === 0
                  }
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue
                        placeholder={
                          props.isLoadingModels
                            ? t('Loading models...')
                            : t('Select a model')
                        }
                      />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    {bananaModels.length > 0 ? (
                      <SelectGroup>
                        <SelectLabel>Nano Banana</SelectLabel>
                        {bananaModels.map((model) => (
                          <SelectItem key={model.value} value={model.value}>
                            {model.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    ) : null}
                    {gptImageModels.length > 0 ? (
                      <SelectGroup>
                        <SelectLabel>GPT Image</SelectLabel>
                        {gptImageModels.map((model) => (
                          <SelectItem key={model.value} value={model.value}>
                            {model.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    ) : null}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>

        <FormField
          control={form.control}
          name='prompt'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Prompt')}</FormLabel>
              <FormControl>
                <Textarea
                  {...field}
                  className='min-h-28 resize-y sm:min-h-32'
                  placeholder={t('Describe the image you want to create')}
                  disabled={props.disabled}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <GenerationOptions
          control={form.control}
          disabled={props.disabled}
          family={family}
          model={selectedModel}
        />

        {props.models.length === 0 && !props.isLoadingModels ? (
          <p className='text-muted-foreground rounded-lg border border-dashed p-3 text-sm'>
            {t(
              'No Nano Banana or GPT Image models are available in this group.'
            )}
          </p>
        ) : null}

        <Button
          type='submit'
          size='lg'
          className='w-full'
          disabled={props.disabled || props.models.length === 0}
        >
          {props.disabled ? (
            <LoaderCircle className='animate-spin' aria-hidden='true' />
          ) : (
            <Sparkles aria-hidden='true' />
          )}
          {props.disabled ? t('Generating...') : t('Generate image')}
        </Button>
      </form>
    </Form>
  )
}
