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
import { useTranslation } from 'react-i18next'

import { getCurrencyLabel } from '@/lib/currency'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { DEFAULT_TOKEN_UNIT } from '../constants'
import { useBillingTime } from '../hooks/use-billing-time'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPriceUnitLabelKey,
  getDynamicPricingSummary,
  isUnconfiguredTaskUsageModel,
} from '../lib/dynamic-price'
import { isTokenBasedModel } from '../lib/model-helpers'
import { formatPrice, formatRequestPrice } from '../lib/price'
import type { PricingModel, TokenUnit } from '../types'

export type ModelPriceCellOptions = {
  tokenUnit?: TokenUnit
  priceRate?: number
  usdExchangeRate?: number
  showRechargePrice?: boolean
  selectedGroup?: string
}

function withoutCurrencySymbol(value: string, customSymbol: string): string {
  let result = value.replaceAll('$', '').replaceAll('¥', '')
  if (customSymbol) result = result.split(customSymbol).join('')
  return result
}

export function ModelPriceCell(props: {
  model: PricingModel
  options?: ModelPriceCellOptions
  showExpression?: boolean
}) {
  const { t } = useTranslation()
  const billingTime = useBillingTime(props.model.billing_expr)
  const currency = useSystemConfigStore((state) => state.config.currency)
  const currencyLabel =
    currency.quotaDisplayType === 'TOKENS' ? 'USD' : getCurrencyLabel()
  const options = props.options ?? {}
  const tokenUnit = options.tokenUnit ?? DEFAULT_TOKEN_UNIT
  const tokenUnitLabel = tokenUnit === 'K' ? '1K' : '1M'
  const dynamic = getDynamicPricingSummary(props.model, {
    ...options,
    now: billingTime === undefined ? undefined : new Date(billingTime),
    tokenUnit,
    groupRatioMultiplier: getDynamicDisplayGroupRatio(
      props.model,
      options.selectedGroup
    ),
  })
  let metrics: Array<{ label: string; value: string }>
  let caption = t('{{currency}} / {{unit}} tokens', {
    currency: currencyLabel,
    unit: tokenUnitLabel,
  })

  if (dynamic) {
    if (dynamic.isSpecialExpression) {
      return (
        <span className='block max-w-full min-w-0'>
          <span className='text-muted-foreground block truncate text-sm'>
            {t('Special billing expression')}
          </span>
          {props.showExpression !== false && (
            <code className='text-muted-foreground mt-1 line-clamp-2 block text-xs break-all whitespace-normal'>
              {dynamic.rawExpression}
            </code>
          )}
        </span>
      )
    }
    if (dynamic.requestPriceEntry) {
      metrics = [
        {
          label: t(
            dynamic.tier &&
              'imageCount' in dynamic.tier &&
              dynamic.tier.imageCount
              ? 'Per image'
              : 'Per-request'
          ),
          value: withoutCurrencySymbol(
            dynamic.requestPriceEntry.formatted,
            currency.customCurrencySymbol
          ),
        },
      ]
      caption = `${currencyLabel} / ${t(dynamic.tier && 'imageCount' in dynamic.tier && dynamic.tier.imageCount ? 'image' : 'request')}`
    } else {
      metrics = dynamic.primaryEntries.slice(0, 2).map((entry) => {
        return {
          label:
            entry.labelKind === 'schema'
              ? entry.shortLabel
              : t(entry.shortLabel),
          value: withoutCurrencySymbol(
            entry.formattedRange ?? entry.formatted,
            currency.customCurrencySymbol
          ),
        }
      })
      if (metrics.length === 0) {
        const expression = dynamic.rawExpression
        const hasInputZero = /\bp\s*\*\s*0(?:\D|$)/.test(expression)
        const hasOutputZero = /\bc\s*\*\s*0(?:\D|$)/.test(expression)
        if (hasInputZero || hasOutputZero) {
          metrics = [
            ...(hasInputZero ? [{ label: t('Input'), value: '0' }] : []),
            ...(hasOutputZero ? [{ label: t('Output'), value: '0' }] : []),
          ]
        } else {
          return (
            <span className='text-muted-foreground text-sm'>
              {t('Dynamic Pricing')}
            </span>
          )
        }
      }
    }
    if (dynamic.isTaskUsage && dynamic.primaryEntries[0]) {
      const unit = getDynamicPriceUnitLabelKey(dynamic.primaryEntries[0])
      caption = `${currencyLabel} /${unit ? t(unit) : t('unit')}`
    } else if (dynamic.tierCount > 1) {
      caption += ` · ${t('{{count}} tiers', { count: dynamic.tierCount })}`
    }
  } else {
    if (isUnconfiguredTaskUsageModel(props.model)) {
      return (
        <span className='text-muted-foreground text-sm'>
          {t('Task pricing not configured')}
        </span>
      )
    }
    const tokenBased = isTokenBasedModel(props.model)
    if (
      !Number.isFinite(
        tokenBased ? props.model.model_ratio : props.model.model_price
      )
    ) {
      return (
        <span className='text-muted-foreground text-sm'>
          {t('Unset price')}
        </span>
      )
    }
    if (tokenBased) {
      metrics = [
        {
          label: t('Input'),
          value: withoutCurrencySymbol(
            formatPrice(
              props.model,
              'input',
              tokenUnit,
              options.showRechargePrice,
              options.priceRate,
              options.usdExchangeRate,
              options.selectedGroup
            ),
            currency.customCurrencySymbol
          ),
        },
        {
          label: t('Output'),
          value: withoutCurrencySymbol(
            formatPrice(
              props.model,
              'output',
              tokenUnit,
              options.showRechargePrice,
              options.priceRate,
              options.usdExchangeRate,
              options.selectedGroup
            ),
            currency.customCurrencySymbol
          ),
        },
      ]
    } else {
      metrics = [
        {
          label: t('Per-request'),
          value: withoutCurrencySymbol(
            formatRequestPrice(
              props.model,
              options.showRechargePrice,
              options.priceRate,
              options.usdExchangeRate,
              options.selectedGroup
            ),
            currency.customCurrencySymbol
          ),
        },
      ]
      caption = `${currencyLabel} / ${t('request')}`
    }
  }
  if (dynamic?.isTimePricing) caption += ` · ${t('Current time period')}`
  return (
    <span className='block w-full max-w-full min-w-0 space-y-1.5'>
      <span
        className={metrics.length > 1 ? 'grid grid-cols-2 gap-x-4' : 'grid'}
      >
        {metrics.map((metric) => (
          <span
            key={metric.label}
            className='flex min-w-0 flex-col items-start gap-y-0.5 sm:flex-row sm:flex-wrap sm:items-baseline sm:gap-x-1.5'
          >
            <span
              className='text-muted-foreground min-w-0 truncate text-xs font-normal'
              title={metric.label}
            >
              {metric.label}
            </span>
            <span
              className='min-w-0 font-mono text-sm break-words whitespace-normal tabular-nums'
              title={metric.value}
            >
              {metric.value}
            </span>
          </span>
        ))}
      </span>
      <span
        className='text-muted-foreground block text-xs font-normal break-words whitespace-normal sm:truncate'
        title={caption}
      >
        {caption}
      </span>
    </span>
  )
}
