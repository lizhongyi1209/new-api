import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import {
  invalidateModelPricing,
  saveTaskPluginPricing,
} from '@/features/model-pricing/api'
import type { PricingCurrency } from '@/features/model-pricing/currency'
import {
  combineBillingExpr,
  splitBillingExprAndRequestRules,
} from '@/features/pricing/lib/billing-expr'
import {
  createDefaultTaskVisualConfig,
  generateTaskExprFromConfig,
} from '@/features/pricing/lib/task-expr'
import type { BillingPluginVariant } from '@/features/pricing/types'
import { handleServerError } from '@/lib/handle-server-error'

import { TaskUsagePricingEditor } from './task-usage-pricing-editor'

type TaskPluginPricingSectionProps = {
  modelName: string
  variants: BillingPluginVariant[]
  currency: PricingCurrency
}

function TaskPluginPricingDraft(props: {
  modelName: string
  variant: BillingPluginVariant
  currency: PricingCurrency
}) {
  const { t } = useTranslation()
  const client = useQueryClient()
  const [resetOpen, setResetOpen] = useState(false)
  const initial = splitBillingExprAndRequestRules(
    props.variant.billing_expr ??
      generateTaskExprFromConfig(
        createDefaultTaskVisualConfig(props.variant.billing_usage_schema),
        props.variant.billing_usage_schema
      )
  )
  const [billingExpr, setBillingExpr] = useState(initial.billingExpr)
  const [requestRuleExpr, setRequestRuleExpr] = useState(
    initial.requestRuleExpr
  )
  const update = useMutation({
    mutationFn: saveTaskPluginPricing,
    onSuccess: async () => {
      await invalidateModelPricing(client)
      toast.success(t('Plugin price saved'))
      setResetOpen(false)
    },
    onError: (error) => handleServerError(error),
  })
  const expectedVersion = props.variant.version ?? ''
  const expression = combineBillingExpr(billingExpr, requestRuleExpr)
  const dirty = expression !== (props.variant.billing_expr ?? '')

  return (
    <div className='space-y-4'>
      <TaskUsagePricingEditor
        currency={props.currency}
        billingExpr={billingExpr}
        requestRuleExpr={requestRuleExpr}
        usageSchema={props.variant.billing_usage_schema}
        usageExamples={props.variant.billing_usage_examples}
        onBillingExprChange={setBillingExpr}
        onRequestRuleExprChange={setRequestRuleExpr}
      />
      <div className='flex flex-wrap justify-end gap-2'>
        {props.variant.billing_expr && (
          <Button
            type='button'
            variant='outline'
            disabled={update.isPending || !expectedVersion}
            onClick={() => setResetOpen(true)}
          >
            {t('Reset plugin price')}
          </Button>
        )}
        <Button
          type='button'
          disabled={
            update.isPending || !expectedVersion || !expression || !dirty
          }
          onClick={() =>
            update.mutate({
              plugin_key: props.variant.plugin_key,
              model_name: props.modelName,
              expected_version: expectedVersion,
              billing_expr: expression,
            })
          }
        >
          {update.isPending ? t('Saving...') : t('Save plugin price')}
        </Button>
      </div>
      <ConfirmDialog
        open={resetOpen}
        onOpenChange={setResetOpen}
        title={t('Reset plugin price?')}
        desc={t(
          'Plugin requests will be unavailable until a new price is saved.'
        )}
        destructive
        isLoading={update.isPending}
        confirmText={t('Reset')}
        handleConfirm={() =>
          update.mutate({
            plugin_key: props.variant.plugin_key,
            model_name: props.modelName,
            expected_version: expectedVersion,
            billing_expr: '',
            reset: true,
          })
        }
      />
    </div>
  )
}

export function TaskPluginPricingSection(props: TaskPluginPricingSectionProps) {
  const { t } = useTranslation()
  const [selectedPluginKey, setSelectedPluginKey] = useState('')
  const selectedVariant =
    props.variants.find(
      (variant) => variant.plugin_key === selectedPluginKey
    ) ?? props.variants[0]
  if (!selectedVariant) return null

  return (
    <section
      className='space-y-3 border-t pt-4'
      aria-label={t('Task plugin pricing')}
    >
      <div>
        <h4 className='text-sm font-medium'>{t('Task plugin pricing')}</h4>
        <p className='text-muted-foreground text-xs'>
          {t('Plugin prices are saved separately from ordinary model prices.')}
        </p>
      </div>
      {props.variants.length > 1 && (
        <div
          role='group'
          aria-label={t('Provider')}
          className='flex flex-wrap gap-2'
        >
          {props.variants.map((variant) => (
            <Button
              key={variant.plugin_key}
              type='button'
              size='sm'
              variant={
                selectedVariant.plugin_key === variant.plugin_key
                  ? 'secondary'
                  : 'outline'
              }
              aria-pressed={selectedVariant.plugin_key === variant.plugin_key}
              onClick={() => setSelectedPluginKey(variant.plugin_key)}
            >
              {variant.plugin_name}
            </Button>
          ))}
        </div>
      )}
      <TaskPluginPricingDraft
        key={`${props.modelName}:${selectedVariant.plugin_key}:${selectedVariant.version}`}
        modelName={props.modelName}
        variant={selectedVariant}
        currency={props.currency}
      />
    </section>
  )
}
