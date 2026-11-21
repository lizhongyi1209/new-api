import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { toIntlLocale } from '@/i18n/languages'
import { formatQuotaWithCurrency } from '@/lib/currency'

import type { Period } from './api'
import { estimatedProfit, pricingGroupKey, type Markup } from './period'

const markupSchema = z.object({
  base: z.number().positive().max(1000000),
  overrides: z.array(
    z
      .string()
      .refine(
        (value) =>
          !value ||
          (Number.isFinite(Number(value)) &&
            Number(value) > 0 &&
            Number(value) <= 1000000)
      )
  ),
})
type MarkupForm = z.infer<typeof markupSchema>

export function ProfitDialog(props: {
  period: Period
  value: Markup
  onChange: (value: Markup) => void
  onClose: () => void
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  // Keep the edited group rows stable while the dashboard refreshes.
  const [period] = useState(props.period)
  const groups = [...period.pricing_groups].sort((a, b) =>
    pricingGroupKey(a).localeCompare(pricingGroupKey(b))
  )
  const form = useForm<MarkupForm>({
    resolver: zodResolver(markupSchema),
    defaultValues: {
      base: props.value.base,
      overrides: groups.map((group) =>
        String(props.value.overrides[pricingGroupKey(group)] ?? '')
      ),
    },
  })
  const values = form.watch()
  const overrides = { ...props.value.overrides }
  groups.forEach((group, index) => {
    const key = pricingGroupKey(group)
    if (values.overrides[index]) {
      overrides[key] = Number(values.overrides[index])
    } else delete overrides[key]
  })
  const profit = estimatedProfit(period.spent_quota, groups, {
    base: values.base,
    overrides,
  })
  const amount = (quota: number | null) =>
    formatQuotaWithCurrency(quota, {
      showSymbol: false,
      abbreviate: false,
      digitsLarge: 2,
      digitsSmall: 2,
      locale,
    })
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
      title={t('Profit estimate')}
      contentClassName='ledger-dialog sm:max-w-lg'
    >
      <form
        className='space-y-5'
        onSubmit={form.handleSubmit((value) => {
          props.onChange({ base: value.base, overrides })
          props.onClose()
        })}
      >
        <div className='space-y-2'>
          <Label htmlFor='reseller-markup'>{t('Markup multiplier')}</Label>
          <Input
            id='reseller-markup'
            type='number'
            step='any'
            {...form.register('base', { valueAsNumber: true })}
          />
        </div>
        <p className='ledger-muted'>
          {t('Sale multiplier ÷ supply multiplier. Estimate only.')}
        </p>
        <details>
          <summary>{t('Group overrides')}</summary>
          <div className='mt-4 space-y-3'>
            {groups.map((group, index) => (
              <div key={pricingGroupKey(group)} className='space-y-1'>
                <Label htmlFor={`markup-${index}`}>
                  {group.user_group} / {group.billing_group}
                </Label>
                <Input
                  id={`markup-${index}`}
                  type='number'
                  step='any'
                  placeholder={String(values.base)}
                  {...form.register(`overrides.${index}`)}
                />
              </div>
            ))}
          </div>
        </details>
        <dl className='ledger-readout'>
          <dt>{t('Net consumption')}</dt>
          <dd>{amount(period.spent_quota)}</dd>
          <dt>{t('Estimated supply cost')}</dt>
          <dd>
            {amount(profit === null ? null : period.spent_quota - profit)}
          </dd>
          <dt>{t('Estimated profit')}</dt>
          <dd>{amount(profit)}</dd>
        </dl>
        {Object.keys(form.formState.errors).length > 0 && (
          <p role='alert' className='text-red-700'>
            {t('Enter a positive multiplier')}
          </p>
        )}
        <div className='flex justify-end'>
          <Button type='submit'>{t('Apply estimate')}</Button>
        </div>
      </form>
    </Dialog>
  )
}
