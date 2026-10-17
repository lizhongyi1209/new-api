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
import { useEffect, useId, useMemo, useRef, useState } from 'react'
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
import { Label } from '@/components/ui/label'
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
  getImageModelFamily,
  getImageModelOption,
  type ImageModelTier,
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
  const tierId = useId()
  const [references, setReferences] = useState<ReferenceImage[]>([])
  const previewUrlsRef = useRef(new Set<string>())
  const form = useForm<ImageGenerationFormValue>({
    defaultValues,
    resolver: zodResolver(imageGenerationSchema),
  })
  const selectedGroup = useWatch({ control: form.control, name: 'group' })
  const selectedModel = useWatch({ control: form.control, name: 'model' })
  const selectedOption = getImageModelOption(selectedModel)
  const selectedSeriesId = selectedOption?.seriesId ?? ''
  const family = selectedOption?.family ?? null
  const modelSeriesOptions = useMemo(() => {
    const seen = new Set<string>()
    return props.models.flatMap((model) => {
      const option = getImageModelOption(model.value)
      if (!option || seen.has(option.seriesId)) return []
      seen.add(option.seriesId)
      return [
        {
          family: option.family,
          label: option.seriesLabel,
          value: option.seriesId,
        },
      ]
    })
  }, [props.models])
  const bananaModels = modelSeriesOptions.filter(
    (model) => model.family === 'banana'
  )
  const gptImageModels = modelSeriesOptions.filter(
    (model) => model.family === 'gpt-image'
  )
  const availableTiers = new Set<ImageModelTier>()
  for (const model of props.models) {
    const option = getImageModelOption(model.value)
    if (option?.seriesId === selectedSeriesId) availableTiers.add(option.tier)
  }
  const tierOptions: Array<
    SelectOption & { disabled: boolean; value: ImageModelTier }
  > = [
    {
      disabled: !availableTiers.has('special'),
      label: t('Special price'),
      value: 'special',
    },
    {
      disabled: !availableTiers.has('premium'),
      label: t('Premium'),
      value: 'premium',
    },
    {
      disabled: !availableTiers.has('enterprise'),
      label: t('Enterprise'),
      value: 'enterprise',
    },
  ]
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

  const selectSeries = (seriesId: string) => {
    const matchingModels = props.models.filter(
      (model) => getImageModelOption(model.value)?.seriesId === seriesId
    )
    const preferredModel =
      matchingModels.find(
        (model) =>
          getImageModelOption(model.value)?.tier === selectedOption?.tier
      ) ?? matchingModels[0]
    if (preferredModel) selectModel(preferredModel.value)
  }

  const selectTier = (tier: string) => {
    const matchingModel = props.models.find((model) => {
      const option = getImageModelOption(model.value)
      return option?.seriesId === selectedSeriesId && option.tier === tier
    })
    if (matchingModel) {
      form.setValue('model', matchingModel.value, { shouldValidate: true })
    }
  }

  return (
    <Form {...form}>
      <form
        className='flex flex-1 flex-col gap-4 sm:gap-5'
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
              <FormItem className='sm:col-span-2 xl:col-span-1'>
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
            render={() => (
              <FormItem>
                <FormLabel>{t('Model')}</FormLabel>
                <Select
                  items={modelSeriesOptions}
                  value={selectedSeriesId}
                  onValueChange={(value) => value && selectSeries(value)}
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

          <div className='grid gap-2'>
            <Label htmlFor={tierId}>{t('Tier')}</Label>
            <Select
              items={tierOptions}
              value={selectedOption?.tier ?? ''}
              onValueChange={(value) => value && selectTier(value)}
              disabled={props.disabled || selectedSeriesId === ''}
            >
              <SelectTrigger id={tierId} className='w-full'>
                <SelectValue placeholder={t('Tier')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {tierOptions.map((tier) => (
                    <SelectItem
                      key={tier.value}
                      value={tier.value}
                      disabled={tier.disabled}
                    >
                      {tier.label}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
        </div>

        <FormField
          control={form.control}
          name='prompt'
          render={({ field }) => (
            <FormItem className='min-h-36 flex-1 grid-rows-[auto_minmax(7rem,1fr)_auto]'>
              <FormLabel>{t('Prompt')}</FormLabel>
              <FormControl>
                <Textarea
                  {...field}
                  className='h-full min-h-28 resize-y sm:min-h-32'
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
