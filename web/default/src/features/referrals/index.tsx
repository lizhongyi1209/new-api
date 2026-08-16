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
import { useQuery } from '@tanstack/react-query'
import { CircleDollarSign, UsersRound } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { formatCurrencyFromUSD } from '@/lib/currency'

import { getReferralProgramSummary } from './api'
import { ReferralLevelCard } from './components/referral-level-card'

const currencyOptions = {
  digitsLarge: 2,
  digitsSmall: 2,
  abbreviate: false,
} as const

const referralLevels = [1, 2, 3]

export function ReferralProgram() {
  const { t } = useTranslation()
  const summaryQuery = useQuery({
    queryKey: ['referrals', 'summary'],
    queryFn: getReferralProgramSummary,
    staleTime: 60 * 1000,
    retry: false,
  })

  let content: ReactNode
  if (summaryQuery.isLoading) {
    content = (
      <>
        <div className='grid gap-3 sm:grid-cols-2'>
          <Skeleton className='h-28 w-full' />
          <Skeleton className='h-28 w-full' />
        </div>
        <div className='grid gap-3'>
          {referralLevels.map((level) => (
            <Skeleton key={level} className='h-20 w-full' />
          ))}
        </div>
      </>
    )
  } else if (summaryQuery.isError || !summaryQuery.data) {
    content = (
      <Alert variant='destructive'>
        <AlertTitle>{t('Unable to load referral program')}</AlertTitle>
        <AlertDescription>
          {t('Please refresh the page and try again.')}
        </AlertDescription>
      </Alert>
    )
  } else {
    content = (
      <>
        <div className='grid gap-3 sm:grid-cols-2'>
          <Card>
            <CardHeader className='flex-row items-center justify-between'>
              <CardTitle className='text-sm font-medium'>
                {t('Total Referrals')}
              </CardTitle>
              <UsersRound
                className='text-muted-foreground size-4'
                aria-hidden='true'
              />
            </CardHeader>
            <CardContent>
              <p className='text-2xl font-semibold tabular-nums'>
                {summaryQuery.data.total_count}
              </p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className='flex-row items-center justify-between'>
              <CardTitle className='text-sm font-medium'>
                {t('Total Top-up')}
              </CardTitle>
              <CircleDollarSign
                className='text-muted-foreground size-4'
                aria-hidden='true'
              />
            </CardHeader>
            <CardContent>
              <p className='text-2xl font-semibold tabular-nums'>
                {formatCurrencyFromUSD(
                  summaryQuery.data.total_top_up,
                  currencyOptions
                )}
              </p>
            </CardContent>
          </Card>
        </div>

        <div className='grid gap-3'>
          {summaryQuery.data.levels.map((level) => (
            <ReferralLevelCard key={level.level} summary={level} />
          ))}
        </div>
      </>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Referral Program')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto grid max-w-5xl gap-4'>{content}</div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
