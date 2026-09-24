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
import { BILLING_PRICING_VARS, type ParsedTier } from './billing-expr'
import {
  readTimeTokenPricing,
  type TimeTokenTier,
} from './billing-expression/display'

function timePricingTier(tier: TimeTokenTier): ParsedTier {
  const result: ParsedTier = {
    label: tier.label,
    conditions: tier.conditions,
    conditionText: tier.conditionText,
    requestPrice: 0,
    billingUnit: tier.billingUnit,
    fixedPrice: tier.fixedPrice,
    imageCount: tier.imageCount,
  }
  for (const variable of BILLING_PRICING_VARS) {
    if (variable.field && Object.hasOwn(tier.prices, variable.key)) {
      result[variable.field] =
        tier.prices[variable.key as keyof typeof tier.prices]
    }
  }
  return result
}

/** Readonly prices for supported time branches. No request simulation or settlement. */
export function readTimePricingTiers(source: string, now?: Date) {
  const pricing = readTimeTokenPricing(source, now)
  if (!pricing) return null
  return {
    tiers: pricing.tiers.map(timePricingTier),
    currentTiers: pricing.currentTiers.map(timePricingTier),
  }
}
