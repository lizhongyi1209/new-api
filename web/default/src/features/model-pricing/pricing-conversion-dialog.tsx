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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { CopyButton } from '@/components/copy-button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { BILLING_VARS } from '@/features/pricing/lib/billing-expr'
import { formatBillingCondition } from '@/features/pricing/lib/billing-expression/condition-display'
import {
  readTokenTierChain,
  type TokenTier,
} from '@/features/pricing/lib/billing-expression/display'
import { compileBillingExpression } from '@/features/pricing/lib/billing-expression/parser'
import { visitExpression } from '@/features/pricing/lib/billing-expression/types'
import {
  buildPreviewRows,
  createInitialLaneState,
} from '@/features/system-settings/models/model-pricing-core'

import type { ModelPricingConversion } from './api'
import { formatPricingAmount, type PricingCurrency } from './currency'
import { pricingRow } from './pricing'

export function PricingConversionDialog(props: {
  modelName: string
  preview: ModelPricingConversion
  currency: PricingCurrency
  onCancel: () => void
  onConfirm: () => void
}) {
  const { t, i18n } = useTranslation()
  const before = pricingRow(props.modelName, props.preview.effective)
  const lanes = createInitialLaneState(before)
  const beforeRows = buildPreviewRows(
    before,
    before.billingMode ?? 'per-token',
    '',
    '',
    lanes.promptPrice,
    lanes.prices,
    lanes.enabled,
    t,
    props.currency
  )
  const details = props.preview.billing_details
  for (const [key, price] of [
    ['audio', details?.audio_input_price],
    ['audioCompletion', details?.audio_output_price],
  ] as const) {
    if (price === undefined) continue
    const row = beforeRows.find((row) => row.key === key)
    if (row) row.value = formatPricingAmount(price, props.currency)
  }
  if (
    props.preview.cache_write_mode === 'claude_ttl' &&
    before.billingMode === 'per-token'
  ) {
    beforeRows.push({
      key: 'cc1h',
      label: t('Cache create (1h) price'),
      value: formatPricingAmount(
        Number(lanes.prices.createCache) * (6 / 3.75),
        props.currency
      ),
    })
  }
  const expression = props.preview.expression ?? ''
  const compiled = useMemo(
    () => compileBillingExpression(expression),
    [expression]
  )
  const tiers = useMemo(() => {
    const values: TokenTier[] = []
    if (compiled.status === 'ready') {
      visitExpression(compiled.ast, (node) => {
        if (node.kind !== 'call' || node.name !== 'tier') return
        const tier = readTokenTierChain(node)?.[0]
        if (tier) values.push(tier)
      })
    }
    return values
  }, [compiled])
  const imageCount =
    compiled.status === 'ready' && compiled.variables.has('image_count')
  const rules = details?.request_rules ?? []

  return (
    <ConfirmDialog
      open
      onOpenChange={(open) => {
        if (!open) props.onCancel()
      }}
      title={t('Preview pricing conversion')}
      desc={t(
        'Review the prices and billing expression. Confirming updates the draft; save model pricing to apply it.'
      )}
      confirmText={t('Apply to draft')}
      handleConfirm={props.onConfirm}
      className='max-h-[90vh] overflow-y-auto data-[size=default]:sm:max-w-4xl'
    >
      <p className='font-mono text-sm break-all'>{props.modelName}</p>
      <div className='grid min-w-0 gap-4 md:grid-cols-2'>
        <section
          aria-label={t('Before conversion')}
          className='min-w-0 space-y-3 rounded-lg border p-4'
        >
          <h3 className='font-semibold'>{t('Before conversion')}</h3>
          <dl className='space-y-2'>
            {beforeRows
              .filter((row) => row.value !== t('Empty'))
              .map((row) => (
                <div
                  key={row.key}
                  className='flex flex-wrap justify-between gap-2 text-sm'
                >
                  <dt className='text-muted-foreground'>{row.label}</dt>
                  <dd className='font-mono tabular-nums'>
                    {row.value} /{' '}
                    {before.billingMode === 'per-request'
                      ? t(details?.image_count ? 'image' : 'request')
                      : t('1M token')}
                  </dd>
                </div>
              ))}
          </dl>
        </section>
        <section
          aria-label={t('After conversion')}
          className='bg-primary/5 min-w-0 space-y-3 rounded-lg border p-4'
        >
          <h3 className='font-semibold'>{t('After conversion')}</h3>
          {tiers.map((tier) => (
            <div key={tier.label} className='space-y-2'>
              {tiers.length > 1 && (
                <p className='text-sm font-medium'>{tier.label}</p>
              )}
              <dl className='space-y-2'>
                {tier.billingUnit === 'request' ? (
                  <div className='flex flex-wrap justify-between gap-2 text-sm'>
                    <dt>{t('Price per request')}</dt>
                    <dd className='font-mono'>
                      {formatPricingAmount(
                        tier.fixedPrice ?? 0,
                        props.currency
                      )}{' '}
                      / {t(imageCount ? 'image' : 'request')}
                    </dd>
                  </div>
                ) : (
                  Object.entries(tier.prices).map(([variable, price]) => (
                    <div
                      key={variable}
                      className='flex flex-wrap justify-between gap-2 text-sm'
                    >
                      <dt className='text-muted-foreground'>
                        {t(
                          BILLING_VARS.find((item) => item.key === variable)
                            ?.label ?? variable
                        )}
                      </dt>
                      <dd className='font-mono tabular-nums'>
                        {formatPricingAmount(price, props.currency)} /{' '}
                        {t('1M token')}
                      </dd>
                    </div>
                  ))
                )}
              </dl>
            </div>
          ))}
          <div className='flex items-center justify-between border-t pt-3'>
            <span className='text-sm'>{t('Billing expression')} (USD)</span>
            <CopyButton value={expression} />
          </div>
          <pre className='bg-background rounded-md border p-3 text-xs break-words whitespace-pre-wrap'>
            <code>{expression}</code>
          </pre>
        </section>
      </div>
      {details?.image_count && (
        <p className='text-muted-foreground text-xs'>
          {t('Reserve requested images; settle returned images.')}
        </p>
      )}
      {rules.map((rule) => (
        <p key={rule.condition} className='text-muted-foreground text-xs'>
          {formatBillingCondition(rule.condition, t, i18n.language) ??
            rule.condition}{' '}
          · ×{rule.multiplier}
        </p>
      ))}
      <Alert>
        <AlertDescription className='text-xs'>
          {t(
            'After conversion, expression reservation and rounding rules apply. Effective unit prices are preserved; individual rounded charges may differ.'
          )}
        </AlertDescription>
      </Alert>
    </ConfirmDialog>
  )
}
