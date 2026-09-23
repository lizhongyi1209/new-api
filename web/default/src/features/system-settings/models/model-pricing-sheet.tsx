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
import { useMutation, useQuery } from '@tanstack/react-query'
import { AlertTriangle, Save } from 'lucide-react'
import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useMemo,
  useState,
  useRef,
  type ReactNode,
} from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { sideDrawerContentClassName } from '@/components/drawer-layout'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
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
import { InputGroup, InputGroupAddon } from '@/components/ui/input-group'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  previewModelPricing,
  previewModelPricingConversion,
  useCanEditModelPricing,
  useModelPricing,
  type ModelPricingConversion,
} from '@/features/model-pricing/api'
import {
  formatPricingAmount,
  getSitePricingCurrency,
  isValidPricingCurrency,
  USD_PRICING_CURRENCY,
} from '@/features/model-pricing/currency'
import { pricingFromDraft, pricingRow } from '@/features/model-pricing/pricing'
import { PricingAmountInput } from '@/features/model-pricing/pricing-amount-input'
import { PricingConversionDialog } from '@/features/model-pricing/pricing-conversion-dialog'
import { PricingCurrencySelector } from '@/features/model-pricing/pricing-currency-selector'
import { DynamicPricingBreakdown } from '@/features/pricing/components/dynamic-pricing-breakdown'
import { combineBillingExpr } from '@/features/pricing/lib/billing-expr'
import { useDebounce } from '@/hooks/use-debounce'
import { handleServerError } from '@/lib/handle-server-error'
import { cn } from '@/lib/utils'
import { usePricingPreferencesStore } from '@/stores/pricing-preferences-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import {
  EMPTY_LANE_ENABLED,
  EMPTY_LANE_PRICES,
  buildPreviewRows,
  createInitialLaneState,
  createModelPricingSchema,
  hasValue,
  laneConfigs,
  numericDraftRegex,
  ratioFieldByLane,
  toNumberOrNull,
  type LaneKey,
  type ModelPricingFormValues,
  type ModelRatioData,
  type PricingMode,
} from './model-pricing-core'
import { PriceInput, PriceLane } from './model-pricing-inputs'
import { formatPricingNumber } from './pricing-format'
import { TaskPluginPricingSection } from './task-plugin-pricing-section'
import { TieredPricingEditor } from './tiered-pricing-editor'

export type { ModelRatioData } from './model-pricing-core'

type ModelPricingSheetProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  editData?: ModelRatioData | null
  onSave?: () => void | Promise<void>
  isSaving?: boolean
}

type ModelPricingEditorPanelProps = Omit<
  ModelPricingSheetProps,
  'open' | 'onOpenChange'
> & {
  className?: string
  onDirtyChange?: (dirty: boolean) => void
  showHeading?: boolean
  showModelName?: boolean
  scrollHeader?: ReactNode
}

export type ModelPricingEditorPanelHandle = {
  commitDraft: () => Promise<ModelRatioData | null>
}

export const ModelPricingSheet = forwardRef<
  ModelPricingEditorPanelHandle,
  ModelPricingSheetProps
>(function ModelPricingSheet(
  { open, onOpenChange, editData, onSave, isSaving },
  ref
) {
  const { t } = useTranslation()
  const title = editData ? t('Edit model pricing') : t('Add model pricing')
  const description = editData?.name || t('New model')

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side='right'
        className={sideDrawerContentClassName('sm:max-w-2xl')}
      >
        <SheetHeader className='sr-only'>
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription>{description}</SheetDescription>
        </SheetHeader>
        <ModelPricingEditorPanel
          ref={ref}
          editData={editData}
          onSave={onSave}
          isSaving={isSaving}
          className='h-full rounded-none border-0'
        />
      </SheetContent>
    </Sheet>
  )
})

export const ModelPricingEditorPanel = forwardRef<
  ModelPricingEditorPanelHandle,
  ModelPricingEditorPanelProps
>(function ModelPricingEditorPanel(
  {
    editData,
    className,
    onSave,
    isSaving,
    onDirtyChange,
    showHeading = true,
    showModelName = true,
    scrollHeader,
  },
  ref
) {
  const { t } = useTranslation()
  const formElementRef = useRef<HTMLFormElement>(null)
  const currencyConfig = useSystemConfigStore((state) => state.config.currency)
  const currencyPreference = usePricingPreferencesStore(
    (state) => state.currency
  )
  const siteCurrency = useMemo(
    () => getSitePricingCurrency(currencyConfig),
    [currencyConfig]
  )
  const pricingCurrency =
    currencyPreference === 'site' && isValidPricingCurrency(siteCurrency)
      ? siteCurrency
      : USD_PRICING_CURRENCY
  const [pricingMode, setPricingMode] = useState<PricingMode>('per-token')
  const [promptPrice, setPromptPrice] = useState('')
  const [lanePrices, setLanePrices] = useState<Record<LaneKey, string>>({
    ...EMPTY_LANE_PRICES,
  })
  const [laneEnabled, setLaneEnabled] = useState<Record<LaneKey, boolean>>({
    ...EMPTY_LANE_ENABLED,
  })
  const [billingExpr, setBillingExpr] = useState('')
  const [requestRuleExpr, setRequestRuleExpr] = useState('')
  const [editorReloadToken, setEditorReloadToken] = useState(0)
  const isEditMode = !!editData
  const pricingConfig = useModelPricing()

  const form = useForm<ModelPricingFormValues>({
    resolver: zodResolver(createModelPricingSchema(t)),
    defaultValues: {
      name: '',
      price: '',
      ratio: '',
      cacheRatio: '',
      createCacheRatio: '',
      completionRatio: '',
      imageRatio: '',
      audioRatio: '',
      audioCompletionRatio: '',
      videoCompletionRatio: '',
    },
  })

  const watchedValues = form.watch()
  const modelName = watchedValues.name.trim()
  const taskPricing = pricingConfig.data?.entries.find(
    (entry) => entry.model_name === modelName
  )
  const usageVariants = taskPricing?.usage_variants ?? []
  const resolvedBillingExpr = billingExpr

  useEffect(() => {
    let initialMode: PricingMode = 'per-token'
    if (editData?.billingMode === 'tiered_expr') initialMode = 'tiered_expr'
    else if (hasValue(editData?.price)) initialMode = 'per-request'
    onDirtyChange?.(
      form.formState.isDirty ||
        pricingMode !== initialMode ||
        billingExpr !== (editData?.billingExpr ?? '') ||
        requestRuleExpr !== (editData?.requestRuleExpr ?? '')
    )
  }, [
    form.formState.isDirty,
    onDirtyChange,
    pricingMode,
    billingExpr,
    requestRuleExpr,
    editData,
  ])

  useEffect(() => {
    const nextLaneState = createInitialLaneState(editData)

    if (editData) {
      form.reset({
        name: editData.name,
        price: editData.price || '',
        ratio: editData.ratio || '',
        cacheRatio: editData.cacheRatio || '',
        createCacheRatio: editData.createCacheRatio || '',
        completionRatio: editData.completionRatio || '',
        imageRatio: editData.imageRatio || '',
        audioRatio: editData.audioRatio || '',
        audioCompletionRatio: editData.audioCompletionRatio || '',
        videoCompletionRatio: editData.videoCompletionRatio || '',
      })
      let nextPricingMode: PricingMode = 'per-token'
      if (editData.billingMode === 'tiered_expr') {
        nextPricingMode = 'tiered_expr'
      } else if (editData.price) {
        nextPricingMode = 'per-request'
      }
      setPricingMode(nextPricingMode)
      setBillingExpr(editData.billingExpr || '')
      setRequestRuleExpr(editData.requestRuleExpr || '')
    } else {
      form.reset({
        name: '',
        price: '',
        ratio: '',
        cacheRatio: '',
        createCacheRatio: '',
        completionRatio: '',
        imageRatio: '',
        audioRatio: '',
        audioCompletionRatio: '',
        videoCompletionRatio: '',
      })
      setPricingMode('per-token')
      setBillingExpr('')
      setRequestRuleExpr('')
    }

    setPromptPrice(nextLaneState.promptPrice)
    setLanePrices(nextLaneState.prices)
    setLaneEnabled(nextLaneState.enabled)
    setEditorReloadToken((token) => token + 1)
  }, [editData, form])

  const setFormValue = (field: keyof ModelPricingFormValues, value: string) => {
    form.setValue(field, value, {
      shouldDirty: true,
      shouldValidate: true,
    })
  }

  const deriveLaneRatio = (
    lane: LaneKey,
    price: string,
    nextPromptPrice = promptPrice,
    nextLanePrices = lanePrices
  ) => {
    const priceNumber = toNumberOrNull(price)
    if (priceNumber === null) return ''

    if (lane === 'audioOutput') {
      const audioInputPrice = toNumberOrNull(nextLanePrices.audioInput)
      if (audioInputPrice === null || audioInputPrice === 0) return ''
      return formatPricingNumber(priceNumber / audioInputPrice)
    }

    const inputPrice = toNumberOrNull(nextPromptPrice)
    if (inputPrice === null || inputPrice === 0) return ''
    return formatPricingNumber(priceNumber / inputPrice)
  }

  const syncLaneRatios = (
    nextPromptPrice = promptPrice,
    nextLanePrices = lanePrices,
    nextLaneEnabled = laneEnabled
  ) => {
    const inputPrice = toNumberOrNull(nextPromptPrice)
    setFormValue(
      'ratio',
      inputPrice !== null ? formatPricingNumber(inputPrice / 2) : ''
    )

    laneConfigs.forEach(({ key }) => {
      const ratioField = ratioFieldByLane[key]
      if (!nextLaneEnabled[key]) {
        setFormValue(ratioField, '')
        return
      }
      setFormValue(
        ratioField,
        deriveLaneRatio(
          key,
          nextLanePrices[key],
          nextPromptPrice,
          nextLanePrices
        )
      )
    })
  }

  const handlePromptPriceChange = (value: string) => {
    if (!numericDraftRegex.test(value)) return
    setPromptPrice(value)
    syncLaneRatios(value, lanePrices, laneEnabled)
  }

  const handleLanePriceChange = (lane: LaneKey, value: string) => {
    if (!numericDraftRegex.test(value)) return
    const nextLanePrices = { ...lanePrices, [lane]: value }
    setLanePrices(nextLanePrices)

    if (laneEnabled[lane]) {
      setFormValue(
        ratioFieldByLane[lane],
        deriveLaneRatio(lane, value, promptPrice, nextLanePrices)
      )
    }

    if (lane === 'audioInput' && laneEnabled.audioOutput) {
      setFormValue(
        'audioCompletionRatio',
        deriveLaneRatio(
          'audioOutput',
          nextLanePrices.audioOutput,
          promptPrice,
          nextLanePrices
        )
      )
    }
  }

  const handleLaneToggle = (lane: LaneKey, checked: boolean) => {
    const nextEnabled = { ...laneEnabled, [lane]: checked }
    let nextPrices = lanePrices

    if (!checked) {
      nextPrices = { ...nextPrices, [lane]: '' }
      setFormValue(ratioFieldByLane[lane], '')
      if (lane === 'audioInput') {
        nextEnabled.audioOutput = false
        nextPrices.audioOutput = ''
        setFormValue('audioCompletionRatio', '')
      }
    }

    setLaneEnabled(nextEnabled)
    setLanePrices(nextPrices)

    if (checked) {
      setFormValue(
        ratioFieldByLane[lane],
        deriveLaneRatio(lane, nextPrices[lane], promptPrice, nextPrices)
      )
    }
  }

  const handleModeChange = (value: string) => {
    const nextMode = value as PricingMode
    setPricingMode(nextMode)
    if (nextMode === 'tiered_expr' && !billingExpr) {
      setBillingExpr('tier("base", p * 0 + c * 0)')
    }
  }

  const canEditPricing = useCanEditModelPricing()
  const draftRequest = useMemo(() => {
    try {
      const modelName = watchedValues.name.trim()
      if (!modelName) return null
      return {
        model_name: modelName,
        pricing: pricingFromDraft({
          ...watchedValues,
          name: modelName,
          billingMode: pricingMode,
          billingExpr: resolvedBillingExpr,
          requestRuleExpr,
        }),
      }
    } catch {
      return null
    }
  }, [watchedValues, pricingMode, resolvedBillingExpr, requestRuleExpr])
  const draftFingerprint = JSON.stringify(draftRequest)
  const currentDraft = useRef(draftFingerprint)
  currentDraft.current = draftFingerprint
  const debouncedFingerprint = useDebounce(draftFingerprint, 250)
  const effectivePreview = useQuery({
    queryKey: ['model-pricing-preview', debouncedFingerprint],
    queryFn: () => previewModelPricing(JSON.parse(debouncedFingerprint)),
    enabled: canEditPricing && debouncedFingerprint !== 'null',
    refetchOnWindowFocus: false,
    retry: false,
    meta: { errorToast: false, errorRedirect: false },
  })
  const [conversionPreview, setConversionPreview] = useState<{
    modelName: string
    fingerprint: string
    result: ModelPricingConversion
  } | null>(null)
  const conversion = useMutation({
    mutationFn: previewModelPricingConversion,
    onError: (error) => handleServerError(error),
  })
  const previewRows = useMemo(() => {
    if (
      effectivePreview.data &&
      draftFingerprint === debouncedFingerprint &&
      draftRequest
    ) {
      const row = pricingRow(
        draftRequest.model_name,
        effectivePreview.data.effective
      )
      const lanes = createInitialLaneState(row)
      const rows = buildPreviewRows(
        row,
        row.billingMode ?? 'per-token',
        resolvedBillingExpr,
        requestRuleExpr,
        lanes.promptPrice,
        lanes.prices,
        lanes.enabled,
        t,
        pricingCurrency
      )
      const details = effectivePreview.data.billing_details
      for (const [key, price] of [
        ['audio', details?.audio_input_price],
        ['audioCompletion', details?.audio_output_price],
      ] as const) {
        if (price === undefined) continue
        const item = rows.find((item) => item.key === key)
        if (item) item.value = formatPricingAmount(price, pricingCurrency)
      }
      return rows
    }
    return buildPreviewRows(
      watchedValues,
      pricingMode,
      resolvedBillingExpr,
      requestRuleExpr,
      promptPrice,
      lanePrices,
      laneEnabled,
      t,
      pricingCurrency
    )
  }, [
    resolvedBillingExpr,
    laneEnabled,
    lanePrices,
    pricingMode,
    promptPrice,
    pricingCurrency,
    requestRuleExpr,
    t,
    watchedValues,
    draftRequest,
    draftFingerprint,
    debouncedFingerprint,
    effectivePreview.data,
  ])

  const warnings = useMemo(() => {
    const nextWarnings: string[] = []
    const hasConflict =
      !!editData?.price &&
      [
        editData.ratio,
        editData.completionRatio,
        editData.cacheRatio,
        editData.createCacheRatio,
        editData.imageRatio,
        editData.audioRatio,
        editData.audioCompletionRatio,
        editData.videoCompletionRatio,
      ].some(hasValue)

    if (hasConflict) {
      nextWarnings.push(
        t(
          'This model has both fixed-price and token-price settings. Saving the current mode will rewrite the conflicting fields.'
        )
      )
    }

    if (
      pricingMode === 'per-token' &&
      toNumberOrNull(promptPrice) === null &&
      laneConfigs.some(
        ({ key }) => laneEnabled[key] && hasValue(lanePrices[key])
      )
    ) {
      nextWarnings.push(
        t('Input price is required before saving dependent prices.')
      )
    }

    if (
      pricingMode === 'per-token' &&
      laneEnabled.audioOutput &&
      !hasValue(lanePrices.audioInput)
    ) {
      nextWarnings.push(t('Audio output price requires an audio input price.'))
    }

    return nextWarnings
  }, [editData, laneEnabled, lanePrices, pricingMode, promptPrice, t])

  const validatePricingValues = useCallback(() => {
    if (
      pricingMode === 'per-token' &&
      toNumberOrNull(promptPrice) === null &&
      laneConfigs.some(
        ({ key }) => laneEnabled[key] && hasValue(lanePrices[key])
      )
    ) {
      form.setError('ratio', {
        message: t('Input price is required before saving dependent prices.'),
      })
      return false
    }

    if (
      pricingMode === 'per-token' &&
      laneEnabled.audioOutput &&
      !hasValue(lanePrices.audioInput)
    ) {
      form.setError('audioRatio', {
        message: t('Audio output price requires an audio input price.'),
      })
      return false
    }

    return true
  }, [form, laneEnabled, lanePrices, pricingMode, promptPrice, t])

  const buildSubmitData = useCallback(
    (values: ModelPricingFormValues) => {
      const data: ModelRatioData = {
        name: values.name.trim(),
        billingMode: pricingMode,
        price: values.price || '',
        ratio: values.ratio || '',
        cacheRatio: values.cacheRatio || '',
        createCacheRatio: values.createCacheRatio || '',
        completionRatio: values.completionRatio || '',
        imageRatio: values.imageRatio || '',
        audioRatio: values.audioRatio || '',
        audioCompletionRatio: values.audioCompletionRatio || '',
        videoCompletionRatio: values.videoCompletionRatio || '',
      }

      if (pricingMode === 'tiered_expr') {
        data.billingExpr = resolvedBillingExpr
        data.requestRuleExpr = requestRuleExpr
      }

      return data
    },
    [resolvedBillingExpr, pricingMode, requestRuleExpr]
  )

  useImperativeHandle(
    ref,
    () => ({
      commitDraft: async () => {
        if (
          pricingMode === 'tiered_expr' &&
          formElementRef.current?.querySelector('[data-billing-invalid="true"]')
        ) {
          toast.error(
            t(
              'Complete the invalid fields before saving or switching modes. The last valid expression is preserved.'
            )
          )
          return null
        }
        for (const input of formElementRef.current?.querySelectorAll<HTMLInputElement>(
          'input[data-pricing-amount]'
        ) ?? []) {
          if (!input.disabled && !input.reportValidity()) return null
        }
        const isValid = await form.trigger()
        if (!isValid || !validatePricingValues()) return null
        return buildSubmitData(form.getValues())
      },
    }),
    [form, pricingMode, t, validatePricingValues, buildSubmitData]
  )

  const showActions = Boolean(onSave)

  return (
    <div
      className={cn(
        'bg-background flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border',
        className
      )}
    >
      {showHeading && (
        <div className='border-b p-4'>
          <div className='flex flex-wrap items-start justify-between gap-3'>
            <div className='min-w-0'>
              <h3 className='truncate text-base font-medium'>
                {isEditMode ? t('Edit model pricing') : t('Add model pricing')}
              </h3>
            </div>
          </div>
        </div>
      )}

      <Form {...form}>
        <form
          ref={formElementRef}
          onSubmit={(event) => event.preventDefault()}
          className='flex min-h-0 flex-1 flex-col'
          autoComplete='off'
        >
          <div className='min-h-0 flex-1 overflow-y-auto p-4 pb-6'>
            {scrollHeader}
            <PricingCurrencySelector siteCurrency={siteCurrency} />
            <div className='grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(220px,260px)]'>
              <FieldGroup>
                {warnings.length > 0 && (
                  <Alert variant='destructive'>
                    <AlertTriangle data-icon='inline-start' />
                    <AlertDescription>
                      <div className='flex flex-col gap-1'>
                        {warnings.map((warning) => (
                          <span key={warning}>{warning}</span>
                        ))}
                      </div>
                    </AlertDescription>
                  </Alert>
                )}

                {showModelName && (
                  <FormField
                    control={form.control}
                    name='name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Model name')}</FormLabel>
                        <FormControl>
                          <Input
                            placeholder={t('gpt-4')}
                            {...field}
                            disabled={isEditMode}
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'The exact model identifier as used in API requests.'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}

                <Tabs
                  value={pricingMode}
                  onValueChange={handleModeChange}
                  className='gap-4'
                >
                  <TabsList className='grid w-full grid-cols-3'>
                    <TabsTrigger value='per-token'>
                      {t('Per-token')}
                    </TabsTrigger>
                    <TabsTrigger value='per-request'>
                      {t('Per-request')}
                    </TabsTrigger>
                    <TabsTrigger value='tiered_expr'>
                      {t('Expression')}
                    </TabsTrigger>
                  </TabsList>

                  <TabsContent value='per-token' className='pt-0'>
                    <FieldGroup className='gap-5'>
                      <Field>
                        <FieldLabel>{t('Input price')}</FieldLabel>
                        <PriceInput
                          aria-label={t('Input price')}
                          currency={pricingCurrency}
                          value={promptPrice}
                          placeholder='3'
                          onChange={handlePromptPriceChange}
                        />
                        <FieldDescription>
                          {t('{{currency}} price per 1M input tokens.', {
                            currency: pricingCurrency.label,
                          })}
                        </FieldDescription>
                      </Field>

                      <div className='grid gap-3 sm:grid-cols-[repeat(auto-fit,minmax(min(100%,320px),1fr))]'>
                        {laneConfigs.map((lane) => {
                          const disabled =
                            lane.key === 'audioOutput' &&
                            (!laneEnabled.audioInput ||
                              !hasValue(lanePrices.audioInput))
                          return (
                            <PriceLane
                              currency={pricingCurrency}
                              key={lane.key}
                              title={t(lane.titleKey)}
                              description={t(lane.descriptionKey)}
                              placeholder={lane.placeholder}
                              value={lanePrices[lane.key]}
                              enabled={laneEnabled[lane.key]}
                              disabled={disabled}
                              onEnabledChange={(checked) =>
                                handleLaneToggle(lane.key, checked)
                              }
                              onChange={(value) =>
                                handleLanePriceChange(lane.key, value)
                              }
                            />
                          )
                        })}
                      </div>
                    </FieldGroup>
                  </TabsContent>

                  <TabsContent value='per-request' className='pt-0'>
                    <FieldGroup className='gap-5'>
                      <FormField
                        control={form.control}
                        name='price'
                        render={({ field }) => (
                          <FormItem className='contents'>
                            <Field>
                              <FieldLabel>{t('Fixed price')}</FieldLabel>
                              <FormControl>
                                <InputGroup>
                                  <InputGroupAddon>
                                    {pricingCurrency.symbol}
                                  </InputGroupAddon>
                                  <PricingAmountInput
                                    aria-label={t('Fixed price')}
                                    grouped
                                    currency={pricingCurrency}
                                    inputMode='decimal'
                                    placeholder='0.01'
                                    {...field}
                                    value={field.value ?? ''}
                                    onChange={(value) => {
                                      if (numericDraftRegex.test(value)) {
                                        field.onChange(value)
                                      }
                                    }}
                                  />
                                  <InputGroupAddon align='inline-end'>
                                    {t('per request')}
                                  </InputGroupAddon>
                                </InputGroup>
                              </FormControl>
                              <FieldDescription>
                                {t(
                                  'Cost in {{currency}} per request, regardless of tokens used.',
                                  { currency: pricingCurrency.label }
                                )}
                              </FieldDescription>
                              <FormMessage />
                            </Field>
                          </FormItem>
                        )}
                      />
                    </FieldGroup>
                  </TabsContent>

                  <TabsContent value='tiered_expr' className='pt-0'>
                    <FieldGroup className='gap-5'>
                      <TieredPricingEditor
                        key={editorReloadToken}
                        currency={pricingCurrency}
                        modelName={watchedValues.name}
                        billingExpr={billingExpr}
                        requestRuleExpr={requestRuleExpr}
                        onBillingExprChange={setBillingExpr}
                        onRequestRuleExprChange={setRequestRuleExpr}
                      />
                    </FieldGroup>
                  </TabsContent>
                </Tabs>
                {usageVariants.length > 0 && (
                  <TaskPluginPricingSection
                    modelName={modelName}
                    variants={usageVariants}
                    currency={pricingCurrency}
                  />
                )}
              </FieldGroup>

              {pricingMode !== 'tiered_expr' && canEditPricing && (
                <div className='space-y-2'>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Review the prices and billing expression. Confirming updates the draft; save model pricing to apply it.'
                    )}
                  </p>
                  <Button
                    type='button'
                    variant='outline'
                    disabled={!draftRequest || isSaving || conversion.isPending}
                    onClick={async () => {
                      if (
                        !draftRequest ||
                        !validatePricingValues() ||
                        !(await form.trigger())
                      ) {
                        return
                      }
                      for (const input of formElementRef.current?.querySelectorAll<HTMLInputElement>(
                        'input[data-pricing-amount]'
                      ) ?? []) {
                        if (!input.disabled && !input.reportValidity()) return
                      }
                      const fingerprint = draftFingerprint
                      const result = await conversion
                        .mutateAsync(draftRequest)
                        .catch(() => null)
                      if (!result || currentDraft.current !== fingerprint) {
                        return
                      }
                      if (!result.expression) {
                        toast.info(
                          t(
                            result.unsupported_reason ??
                              'This expression cannot be edited visually without losing information.'
                          )
                        )
                        return
                      }
                      setConversionPreview({
                        modelName: draftRequest.model_name,
                        fingerprint,
                        result,
                      })
                    }}
                  >
                    {conversion.isPending
                      ? t('Loading...')
                      : t('Preview pricing conversion')}
                  </Button>
                </div>
              )}

              <aside
                className='bg-muted/20 sticky top-0 rounded-lg border'
                aria-label={t('Preview')}
                role='complementary'
              >
                <div className='border-b px-3 py-2'>
                  <div className='text-sm font-medium'>{t('Preview')}</div>
                </div>
                {pricingMode === 'tiered_expr' && (
                  <div className='px-3 py-2'>
                    <DynamicPricingBreakdown
                      compact
                      currentTimePreview
                      billingExpr={combineBillingExpr(
                        resolvedBillingExpr,
                        requestRuleExpr
                      )}
                    />
                  </div>
                )}
                <div className='divide-y'>
                  {previewRows.map((row) => (
                    <div key={row.key} className='grid gap-1 px-3 py-2.5'>
                      <span className='text-muted-foreground text-xs'>
                        {row.label}
                      </span>
                      <span
                        className={cn(
                          'min-w-0 text-sm',
                          row.multiline
                            ? 'font-mono text-xs leading-5 break-words whitespace-pre-wrap'
                            : 'truncate'
                        )}
                      >
                        {row.value}
                      </span>
                    </div>
                  ))}
                </div>
              </aside>
            </div>
          </div>
          {showActions && (
            <div className='bg-background/95 supports-[backdrop-filter]:bg-background/80 shrink-0 border-t p-3 backdrop-blur'>
              <div className='flex flex-col-reverse gap-2 sm:flex-row sm:justify-end'>
                {onSave && (
                  <Button
                    type='button'
                    onClick={onSave}
                    disabled={isSaving}
                    className='w-full sm:w-auto'
                  >
                    <Save data-icon='inline-start' />
                    {isSaving ? t('Saving...') : t('Save model prices')}
                  </Button>
                )}
              </div>
            </div>
          )}
        </form>
      </Form>
      {conversionPreview &&
        conversionPreview.fingerprint === draftFingerprint &&
        conversionPreview.result.expression && (
          <PricingConversionDialog
            modelName={conversionPreview.modelName}
            preview={conversionPreview.result}
            currency={pricingCurrency}
            onCancel={() => setConversionPreview(null)}
            onConfirm={() => {
              if (
                currentDraft.current !== conversionPreview.fingerprint ||
                !conversionPreview.result.expression
              ) {
                return
              }
              for (const field of Object.keys(ratioFieldByLane)) {
                form.setValue(ratioFieldByLane[field as LaneKey], '', {
                  shouldDirty: true,
                })
              }
              form.setValue('price', '', { shouldDirty: true })
              form.setValue('ratio', '', { shouldDirty: true })
              setPricingMode('tiered_expr')
              setBillingExpr(conversionPreview.result.expression)
              setRequestRuleExpr('')
              setEditorReloadToken((token) => token + 1)
              setConversionPreview(null)
            }}
          />
        )}
    </div>
  )
})
