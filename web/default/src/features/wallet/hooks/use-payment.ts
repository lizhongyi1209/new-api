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
import i18next from 'i18next'
import { useState, useCallback, useRef } from 'react'
import { toast } from 'sonner'

import {
  calculateAmount,
  calculateStripeAmount,
  calculateWaffoPancakeAmount,
  requestPayment,
  requestStripePayment,
  isApiSuccess,
} from '../api'
import {
  isStripePayment,
  isWaffoPancakePayment,
  submitPaymentForm,
} from '../lib'

// ============================================================================
// Payment Hook
// ============================================================================

export function usePayment() {
  const [amount, setAmount] = useState<number>(0)
  const [calculationError, setCalculationError] = useState<string | null>(null)
  const [calculating, setCalculating] = useState(false)
  const [processing, setProcessing] = useState(false)
  const calculationIdRef = useRef(0)

  // Calculate payment amount
  const calculatePaymentAmount = useCallback(
    async (topupAmount: number, paymentType: string) => {
      const calculationId = ++calculationIdRef.current

      try {
        setCalculating(true)
        setCalculationError(null)

        const isStripe = isStripePayment(paymentType)
        const isPancake = isWaffoPancakePayment(paymentType)
        let response
        if (isStripe) {
          response = await calculateStripeAmount({ amount: topupAmount })
        } else if (isPancake) {
          response = await calculateWaffoPancakeAmount({ amount: topupAmount })
        } else {
          response = await calculateAmount({ amount: topupAmount })
        }

        if (isApiSuccess(response) && response.data) {
          const calculatedAmount = Number.parseFloat(response.data)
          if (calculationId !== calculationIdRef.current) {
            return 0
          }
          setAmount(calculatedAmount)
          return calculatedAmount
        }

        if (calculationId === calculationIdRef.current) {
          setCalculationError(
            typeof response.data === 'string' && response.data
              ? response.data
              : response.message || 'Payment request failed'
          )
        }
        return 0
      } catch {
        if (calculationId === calculationIdRef.current) {
          setCalculationError('Payment request failed')
        }
        return 0
      } finally {
        if (calculationId === calculationIdRef.current) {
          setCalculating(false)
        }
      }
    },
    []
  )

  // Process payment
  const processPayment = useCallback(
    async (topupAmount: number, paymentType: string) => {
      try {
        setProcessing(true)

        const isStripe = isStripePayment(paymentType)
        const amount = Math.floor(topupAmount)

        const response = isStripe
          ? await requestStripePayment({
              amount,
              payment_method: 'stripe',
            })
          : await requestPayment({
              amount,
              payment_method: paymentType,
            })

        if (!isApiSuccess(response)) {
          const errorMessage =
            typeof response.data === 'string' && response.data
              ? response.data
              : response.message
          toast.error(
            errorMessage && errorMessage !== 'error'
              ? i18next.t(errorMessage)
              : i18next.t('Payment request failed')
          )
          return false
        }

        // Handle Stripe payment
        if (isStripe && response.data?.pay_link) {
          window.open(response.data.pay_link as string, '_blank')
          toast.success(i18next.t('Redirecting to payment page...'))
          return true
        }

        // Handle non-Stripe payment
        if (!isStripe && response.data) {
          const url = (response as unknown as { url?: string }).url
          if (url) {
            submitPaymentForm(url, response.data)
            toast.success(i18next.t('Redirecting to payment page...'))
            return true
          }
        }

        return false
      } catch {
        toast.error(i18next.t('Payment request failed'))
        return false
      } finally {
        setProcessing(false)
      }
    },
    []
  )

  return {
    amount,
    calculationError,
    calculating,
    processing,
    calculatePaymentAmount,
    processPayment,
    setAmount,
  }
}
