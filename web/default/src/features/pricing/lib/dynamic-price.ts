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
import { formatBillingCurrencyFromUSD } from '@/lib/currency'

import { TOKEN_UNIT_DIVISORS } from '../constants'
import type {
  BillingUsageSchema,
  BillingUsageUnit,
  PricingModel,
  TokenUnit,
} from '../types'
import {
  BILLING_PRICING_VARS,
  parseTaskTiersFromExpr,
  parseTiersFromExpr,
  splitBillingExprAndRequestRules,
  tryParseRequestRuleExpr,
  type BillingVar,
  type ParsedTaskTier,
  type ParsedTier,
} from './billing-expr'
import { compileBillingExpression } from './billing-expression/parser'
import { getDisplayGroupRatio } from './model-helpers'
import { readTimePricingTiers } from './time-price'

type DynamicPriceOptions = {
  now?: Date
  tokenUnit: TokenUnit
  showRechargePrice?: boolean
  priceRate?: number
  usdExchangeRate?: number
  groupRatioMultiplier?: number
  usageSchema?: BillingUsageSchema
}

export type DynamicPriceLabelKind = 'i18n' | 'schema'

export type DynamicPriceEntry = {
  key: string
  field: string
  label: string
  shortLabel: string
  labelKind: DynamicPriceLabelKind
  value: number
  formatted: string
  formattedRange?: string
  unit: 'token' | BillingUsageUnit | 'request' | 'image'
  variable?: BillingVar
  description?: string | Record<string, string>
}

export type DynamicPricingTier = ParsedTier | ParsedTaskTier

export type DynamicPricingSummary = {
  tiers: DynamicPricingTier[]
  tier: DynamicPricingTier | null
  tierCount: number
  hasRequestRules: boolean
  isSpecialExpression: boolean
  rawExpression: string
  entries: DynamicPriceEntry[]
  primaryEntries: DynamicPriceEntry[]
  secondaryEntries: DynamicPriceEntry[]
  isTaskUsage: boolean
  isTimePricing: boolean
  requestPriceEntry: { value: number; formatted: string } | null
}

const PRIMARY_DYNAMIC_FIELDS = new Set(['inputPrice', 'outputPrice'])

function getTaskNumberFields(schema: BillingUsageSchema | null | undefined) {
  if (!schema) return []
  return Object.entries(schema)
    .filter((entry) => entry[1].type === 'number' && Boolean(entry[1].unit))
    .sort(([left], [right]) => left.localeCompare(right))
}

export function getTaskUsageQuantityUnitLabelKey(
  unit: BillingUsageUnit | undefined
): string {
  if (unit === 'second') return 's'
  if (unit === 'token') return 'token (unit)'
  if (unit === 'credit') return 'credit'
  return 'unit'
}

export function getTaskUsagePriceUnitLabelKey(
  unit: BillingUsageUnit | undefined
): string {
  if (unit === 'second') return 'second'
  if (unit === 'token') return '1M token'
  if (unit === 'credit') return 'credit'
  return 'unit'
}

function isTaskPricingTier(tier: DynamicPricingTier): tier is ParsedTaskTier {
  return Object.hasOwn(tier, 'unitPrices')
}

export function isDynamicPricingModel(model: PricingModel): boolean {
  return model.billing_mode === 'tiered_expr' && Boolean(model.billing_expr)
}

export function isTaskUsagePricingModel(model: PricingModel): boolean {
  return (
    isDynamicPricingModel(model) &&
    hasTaskUsageSchema(model) &&
    /\bu\s*\(/.test(model.billing_expr ?? '')
  )
}

export function hasTaskUsageSchema(model: PricingModel): boolean {
  return Object.keys(model.billing_usage_schema ?? {}).length > 0
}

export function hasTaskUsageOption(model: PricingModel): boolean {
  return (
    hasTaskUsageSchema(model) ||
    (model.billing_plugin_variants?.some(
      (variant) => Object.keys(variant.billing_usage_schema).length > 0
    ) ?? false)
  )
}

export function isUnconfiguredTaskUsageModel(model: PricingModel): boolean {
  return hasTaskUsageSchema(model) && !isTaskUsagePricingModel(model)
}

export function getDynamicDisplayGroupRatio(
  model: PricingModel,
  selectedGroup?: string
): number {
  return getDisplayGroupRatio(model, selectedGroup)
}

function applyRechargeRate(
  price: number,
  showWithRecharge: boolean,
  priceRate: number,
  usdExchangeRate: number
): number {
  if (!showWithRecharge) return price
  return (price * priceRate) / usdExchangeRate
}

export function formatDynamicUnitPrice(
  valuePerMillionTokens: number,
  options: DynamicPriceOptions
): string {
  const groupRatio = options.groupRatioMultiplier ?? 1
  const priceRate = options.priceRate ?? 1
  const usdExchangeRate = options.usdExchangeRate ?? 1
  const priceUSD =
    (valuePerMillionTokens * groupRatio) /
    TOKEN_UNIT_DIVISORS[options.tokenUnit]
  const displayPrice = applyRechargeRate(
    priceUSD,
    options.showRechargePrice ?? false,
    priceRate,
    usdExchangeRate
  )

  return formatBillingCurrencyFromUSD(displayPrice, {
    digitsLarge: 4,
    digitsSmall: 6,
    abbreviate: false,
  })
}

export function formatDynamicRequestPrice(
  valueUSDPerRequest: number,
  options: DynamicPriceOptions
): string {
  const groupRatio = options.groupRatioMultiplier ?? 1
  const priceRate = options.priceRate ?? 1
  const usdExchangeRate = options.usdExchangeRate ?? 1
  const priceUSD = valueUSDPerRequest * groupRatio
  const displayPrice = applyRechargeRate(
    priceUSD,
    options.showRechargePrice ?? false,
    priceRate,
    usdExchangeRate
  )

  return formatBillingCurrencyFromUSD(displayPrice, {
    digitsLarge: 4,
    digitsSmall: 6,
    abbreviate: false,
  })
}

export function formatTaskUsageUnitPrice(
  valuePerUnit: number,
  options: DynamicPriceOptions
): string {
  const groupRatio = options.groupRatioMultiplier ?? 1
  const priceRate = options.priceRate ?? 1
  const usdExchangeRate = options.usdExchangeRate ?? 1
  const displayPrice = applyRechargeRate(
    valuePerUnit * groupRatio,
    options.showRechargePrice ?? false,
    priceRate,
    usdExchangeRate
  )
  return formatBillingCurrencyFromUSD(displayPrice, {
    digitsLarge: 4,
    digitsSmall: 6,
    abbreviate: false,
  })
}

export function getDynamicPriceUnitLabelKey(
  entry: DynamicPriceEntry
): string | null {
  if (entry.unit === 'second') return 's'
  if (entry.unit === 'count') return 'unit'
  if (entry.unit === 'credit') return 'credit'
  if (entry.unit === 'token' && !entry.variable) return '1M token'
  if (entry.unit === 'request') return 'request'
  if (entry.unit === 'image') return 'image'
  return null
}

export function getDynamicPricingTiers(
  model: PricingModel
): DynamicPricingTier[] {
  if (!isDynamicPricingModel(model)) return []
  const { billingExpr } = splitBillingExprAndRequestRules(
    model.billing_expr || ''
  )
  if (isTaskUsagePricingModel(model)) {
    return parseTaskTiersFromExpr(billingExpr, model.billing_usage_schema)
  }
  const timePricing = readTimePricingTiers(billingExpr)
  if (timePricing) return timePricing.tiers
  return parseTiersFromExpr(billingExpr)
}

export function hasDynamicRequestRules(model: PricingModel): boolean {
  if (!isDynamicPricingModel(model)) return false
  const { requestRuleExpr } = splitBillingExprAndRequestRules(
    model.billing_expr || ''
  )
  if (tryParseRequestRuleExpr(requestRuleExpr || '')?.length) return true
  const compiled = compileBillingExpression(model.billing_expr || '')
  return compiled.status === 'ready' && compiled.requestRules.length > 0
}

export function getDynamicPriceEntries(
  tier: DynamicPricingTier | null,
  options: DynamicPriceOptions
): DynamicPriceEntry[] {
  if (!tier) return []

  if (isTaskPricingTier(tier) && options.usageSchema) {
    const entries: DynamicPriceEntry[] = getTaskNumberFields(
      options.usageSchema
    ).flatMap(([field, definition]) => {
      const value = Number(tier.unitPrices[field])
      if (!Number.isFinite(value) || value <= 0 || !definition.unit) return []
      return [
        {
          key: field,
          field,
          label: field,
          shortLabel: field,
          labelKind: 'schema' as const,
          value,
          formatted: formatTaskUsageUnitPrice(value, options),
          unit: definition.unit,
          description: definition.description,
        },
      ]
    })
    if (tier.constant > 0) {
      entries.push({
        key: 'constant',
        field: 'constant',
        label: 'Base charge',
        shortLabel: 'Base',
        labelKind: 'i18n',
        value: tier.constant,
        formatted: formatTaskUsageUnitPrice(tier.constant, options),
        unit: 'request',
      })
    }
    return entries
  }

  if (
    !isTaskPricingTier(tier) &&
    tier.billingUnit === 'request' &&
    typeof tier.fixedPrice === 'number'
  ) {
    return [
      {
        key: 'fixedPrice',
        field: 'fixedPrice',
        label: tier.imageCount ? 'Per image' : 'Per-request',
        shortLabel: tier.imageCount ? 'Per image' : 'Per-request',
        labelKind: 'i18n',
        value: tier.fixedPrice,
        formatted: formatDynamicRequestPrice(tier.fixedPrice, options),
        unit: tier.imageCount ? 'image' : 'request',
      },
    ]
  }
  return BILLING_PRICING_VARS.flatMap((variable) => {
    if (!variable.field) return []
    const value = Number((tier as ParsedTier)[variable.field])
    if (
      !Number.isFinite(value) ||
      value < 0 ||
      !Object.hasOwn(tier, variable.field)
    ) {
      return []
    }

    return [
      {
        key: variable.key,
        field: variable.field,
        label: variable.label,
        shortLabel: variable.shortLabel,
        labelKind: 'i18n' as const,
        value,
        formatted: formatDynamicUnitPrice(value, options),
        unit: 'token' as const,
        variable,
      },
    ]
  }).sort((a, b) => {
    const aPrimary = PRIMARY_DYNAMIC_FIELDS.has(a.field)
    const bPrimary = PRIMARY_DYNAMIC_FIELDS.has(b.field)
    if (aPrimary !== bPrimary) return aPrimary ? -1 : 1
    return 0
  })
}

export function getDynamicPricingSummary(
  model: PricingModel,
  options: DynamicPriceOptions
): DynamicPricingSummary | null {
  if (!isDynamicPricingModel(model)) return null

  const tiers = getDynamicPricingTiers(model)
  const isTaskUsage = isTaskUsagePricingModel(model)
  const split = splitBillingExprAndRequestRules(model.billing_expr || '')
  const timePricing = isTaskUsage
    ? null
    : readTimePricingTiers(split.billingExpr, options.now ?? new Date())
  let tier: DynamicPricingTier | null = tiers[0] ?? null
  if (timePricing) tier = timePricing.currentTiers[0] ?? null
  else if (isTaskUsage) tier = tiers.at(-1) ?? null
  let entries = getDynamicPriceEntries(tier, {
    ...options,
    usageSchema: model.billing_usage_schema,
  })
  if (isTaskUsage) {
    const ranges = new Map<string, { min: number; max: number }>()
    for (const [field] of getTaskNumberFields(model.billing_usage_schema)) {
      const values = tiers.flatMap((candidate) => {
        if (!isTaskPricingTier(candidate)) return []
        const value = Number(candidate.unitPrices[field])
        return Number.isFinite(value) && value > 0 ? [value] : []
      })
      if (values.length) {
        ranges.set(field, {
          min: Math.min(...values),
          max: Math.max(...values),
        })
      }
    }
    entries = entries.map((entry) => {
      const range = ranges.get(entry.field)
      if (!range || range.min === range.max) return entry
      return {
        ...entry,
        formattedRange: `${formatTaskUsageUnitPrice(range.min, options)} – ${formatTaskUsageUnitPrice(range.max, options)}`,
      }
    })
  }
  const rawExpression = model.billing_expr || ''
  let requestPrice: number | null = null
  if (tier && !isTaskPricingTier(tier)) {
    if (tier.billingUnit === 'request') requestPrice = tier.fixedPrice ?? null
    else if (Number(tier.requestPrice) > 0) {
      requestPrice = Number(tier.requestPrice)
    }
  }

  return {
    tiers,
    tier,
    tierCount: tiers.length,
    hasRequestRules: hasDynamicRequestRules(model),
    isSpecialExpression:
      rawExpression.trim().length > 0 &&
      (tiers.length === 0 || (timePricing !== null && tier === null)),
    rawExpression,
    entries,
    primaryEntries: isTaskUsage
      ? entries.filter((entry) => entry.unit !== 'request')
      : entries.filter(
          (entry) =>
            PRIMARY_DYNAMIC_FIELDS.has(entry.field) ||
            entry.unit === 'request' ||
            entry.unit === 'image'
        ),
    secondaryEntries: isTaskUsage
      ? entries.filter((entry) => entry.unit === 'request')
      : entries.filter(
          (entry) =>
            !PRIMARY_DYNAMIC_FIELDS.has(entry.field) &&
            entry.unit !== 'request' &&
            entry.unit !== 'image'
        ),
    isTaskUsage,
    isTimePricing: timePricing !== null,
    requestPriceEntry:
      requestPrice !== null &&
      Number.isFinite(requestPrice) &&
      requestPrice >= 0
        ? {
            value: requestPrice,
            formatted: formatDynamicRequestPrice(requestPrice, options),
          }
        : null,
  }
}
