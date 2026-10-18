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

import { ConfirmDialog } from '@/components/confirm-dialog'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import {
  buildBatchReferencePlan,
  type BatchReferencePlan,
} from '../lib/batch-references'
import {
  getImageModelFamily,
  getImageModelOption,
  type ImageModelTier,
} from '../lib/image-generation'
import { imageGenerationSchema } from '../lib/schema'
import type {
  BatchCombinationMode,
  BatchReferenceGroup,
  ImageGenerationFormValue,
  ImageResultAction,
  ReferenceImage,
  SelectOption,
} from '../types'
import { BatchReferenceGroups } from './batch-reference-groups'
import { GenerationOptions } from './generation-options'
import { ReferenceImagePicker } from './reference-image-picker'

type GenerationFormProps = {
  disabled: boolean
  editDraft?: { key: number; image: ImageResultAction }
  isLoadingModels: boolean
  models: SelectOption[]
  onSubmit: (
    value: ImageGenerationFormValue,
    referenceSets: ReferenceImage[][]
  ) => void
}

const defaultValues: ImageGenerationFormValue = {
  aspectRatio: '1:1',
  background: 'auto',
  model: '',
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
  const [batchEnabled, setBatchEnabled] = useState(false)
  const [batchMode, setBatchMode] = useState<BatchCombinationMode>('all')
  const [batchGroups, setBatchGroups] = useState<BatchReferenceGroup[]>(() => [
    {
      id: crypto.randomUUID(),
      name: t('Group {{number}}', { number: 1 }),
      images: [],
    },
  ])
  const [pendingBatchValue, setPendingBatchValue] =
    useState<ImageGenerationFormValue | null>(null)
  const previewUrlsRef = useRef(new Set<string>())
  const form = useForm<ImageGenerationFormValue>({
    defaultValues,
    resolver: zodResolver(imageGenerationSchema),
  })
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
  const models = props.models
  const batchPlan: BatchReferencePlan = batchEnabled
    ? buildBatchReferencePlan(references, batchGroups, batchMode)
    : { sets: [references] }
  const referenceSets = batchPlan.sets ?? []
  const batchError = batchEnabled ? batchPlan.error : undefined
  const batchErrorMessages = {
    empty_group: t('Add at least one image to each batch group.'),
    group_count: t('Too many batch groups.'),
    reference_count: t('Each generation can use up to 10 reference images.'),
    upload_count: t('A batch can contain up to 30 uploaded reference images.'),
    uneven_groups: t(
      'Groups must contain one image or the same number of images.'
    ),
    job_count: t('A batch can contain up to 100 generations.'),
  }
  let submitLabel = t('Generate image')
  if (props.disabled) {
    submitLabel = t('Generating...')
  } else if (referenceSets.length > 1) {
    submitLabel = t('Generate {{count}} images', {
      count: referenceSets.length,
    })
  }

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

  useEffect(() => {
    const draft = props.editDraft
    if (!draft) return
    const options = draft.image.options
    const outputFormat = options?.output_format ?? draft.image.outputFormat
    form.reset({
      ...defaultValues,
      model: draft.image.model,
      prompt: draft.image.prompt,
      size:
        options?.size ||
        (getImageModelFamily(draft.image.model) === 'banana' ? '1K' : 'auto'),
      aspectRatio: options?.aspect_ratio || '1:1',
      quality: options?.quality || 'auto',
      background:
        options?.background === 'transparent' ? 'transparent' : 'auto',
      outputFormat:
        outputFormat === 'jpeg' || outputFormat === 'webp'
          ? outputFormat
          : 'png',
      thinkingLevel:
        options?.thinking_level === 'high' ||
        options?.thinking_level === 'minimal'
          ? options.thinking_level
          : 'default',
    })
    setBatchEnabled(false)
    setReferences([
      {
        id: `gallery:${draft.image.task_id}:${draft.image.index}`,
        sourceUrl: draft.image.url,
        previewUrl: draft.image.url,
        name: t('Generated image {{number}}', {
          number: draft.image.index + 1,
        }),
      },
    ])
  }, [form, props.editDraft, t])

  const updateReferences = (nextReferences: ReferenceImage[]) => {
    for (const reference of nextReferences) {
      if (reference.previewUrl.startsWith('blob:')) {
        previewUrlsRef.current.add(reference.previewUrl)
      }
    }
    setReferences(nextReferences)
  }

  const selectModel = (model: string) => {
    form.setValue('model', model, { shouldValidate: true })
    const nextFamily = getImageModelFamily(model)
    form.setValue('size', nextFamily === 'banana' ? '1K' : 'auto')
    form.setValue('outputFormat', 'png')
    form.setValue('background', 'auto')
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
        className='flex flex-col gap-4 sm:gap-5 xl:flex-1'
        onSubmit={form.handleSubmit((value) => {
          if (batchError || referenceSets.length === 0) return
          if (referenceSets.length > 10) {
            setPendingBatchValue(value)
            return
          }
          props.onSubmit(value, referenceSets)
        })}
      >
        <div className='grid gap-3'>
          <FormField
            control={form.control}
            name='model'
            render={() => (
              <FormItem>
                <FormLabel>{t('Model')}</FormLabel>
                <Select
                  modal={false}
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
                    <SelectTrigger className='w-full transition-none'>
                      <SelectValue
                        placeholder={
                          props.isLoadingModels
                            ? t('Loading models...')
                            : t('Select a model')
                        }
                      />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false} animated={false}>
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
              modal={false}
              items={tierOptions}
              value={selectedOption?.tier ?? ''}
              onValueChange={(value) => value && selectTier(value)}
              disabled={props.disabled || selectedSeriesId === ''}
            >
              <SelectTrigger id={tierId} className='w-full transition-none'>
                <SelectValue placeholder={t('Tier')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false} animated={false}>
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

        <div className='flex items-center justify-between gap-3'>
          <Label htmlFor={`${tierId}-batch`}>{t('Batch mode')}</Label>
          <Switch
            id={`${tierId}-batch`}
            checked={batchEnabled}
            onCheckedChange={setBatchEnabled}
            disabled={props.disabled}
          />
        </div>

        {batchEnabled ? (
          <BatchReferenceGroups
            disabled={props.disabled}
            fixed={references}
            groups={batchGroups}
            mode={batchMode}
            onFixedChange={updateReferences}
            onGroupsChange={(groups) => {
              for (const group of groups) {
                for (const image of group.images) {
                  if (image.previewUrl.startsWith('blob:')) {
                    previewUrlsRef.current.add(image.previewUrl)
                  }
                }
              }
              setBatchGroups(groups)
            }}
            onModeChange={setBatchMode}
          />
        ) : (
          <ReferenceImagePicker
            disabled={props.disabled}
            images={references}
            onChange={updateReferences}
          />
        )}

        {batchError ? (
          <p className='text-destructive text-xs' role='alert'>
            {batchErrorMessages[batchError]}
          </p>
        ) : null}

        <FormField
          control={form.control}
          name='prompt'
          render={({ field }) => (
            <FormItem className='min-h-36 grid-rows-[auto_auto] xl:flex-1 xl:grid-rows-[auto_minmax(7rem,1fr)]'>
              <FormLabel>{t('Prompt')}</FormLabel>
              <FormControl>
                <Textarea
                  {...field}
                  className='field-sizing-fixed h-36 max-h-none min-h-28 resize-y sm:h-40 xl:h-full xl:min-h-32 xl:resize-none'
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
            {t('No available models')}
          </p>
        ) : null}

        <Button
          type='submit'
          size='lg'
          className='w-full'
          disabled={
            props.disabled || props.models.length === 0 || Boolean(batchError)
          }
        >
          {props.disabled ? (
            <LoaderCircle className='animate-spin' aria-hidden='true' />
          ) : (
            <Sparkles aria-hidden='true' />
          )}
          {submitLabel}
        </Button>
      </form>
      <ConfirmDialog
        open={pendingBatchValue !== null}
        onOpenChange={(open) => {
          if (!open) setPendingBatchValue(null)
        }}
        title={t('Confirm batch generation')}
        desc={t('This will submit {{count}} separately billed image tasks.', {
          count: referenceSets.length,
        })}
        confirmText={t('Generate {{count}} images', {
          count: referenceSets.length,
        })}
        handleConfirm={() => {
          if (pendingBatchValue && referenceSets.length > 0) {
            props.onSubmit(pendingBatchValue, referenceSets)
          }
          setPendingBatchValue(null)
        }}
      />
    </Form>
  )
}
