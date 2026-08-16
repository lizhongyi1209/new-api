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
import { ChevronDown, Users } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Card } from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatTimestamp } from '@/lib/format'

import { getReferralUsers } from '../api'
import type { ReferralLevelSummary } from '../types'

type ReferralLevelCardProps = {
  summary: ReferralLevelSummary
}

const currencyOptions = {
  digitsLarge: 2,
  digitsSmall: 2,
  abbreviate: false,
} as const

const loadingRows = ['first', 'second', 'third']

export function ReferralLevelCard(props: ReferralLevelCardProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const usersQuery = useQuery({
    queryKey: ['referrals', 'users', props.summary.level],
    queryFn: () => getReferralUsers(props.summary.level),
    enabled: open,
    staleTime: 60 * 1000,
    retry: false,
  })

  let detailsContent: ReactNode
  if (usersQuery.isLoading) {
    detailsContent = (
      <div className='grid gap-3 p-4 sm:p-5'>
        {loadingRows.map((row) => (
          <Skeleton key={row} className='h-10 w-full' />
        ))}
      </div>
    )
  } else if (usersQuery.isError) {
    detailsContent = (
      <div className='p-4 sm:p-5'>
        <Alert variant='destructive'>
          <AlertTitle>{t('Unable to load referrals')}</AlertTitle>
          <AlertDescription>
            {t('Please refresh the page and try again.')}
          </AlertDescription>
        </Alert>
      </div>
    )
  } else if (usersQuery.data?.length) {
    detailsContent = (
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead className='pl-4 sm:pl-5'>{t('Username')}</TableHead>
            <TableHead>{t('Registered At')}</TableHead>
            <TableHead className='pr-4 text-right sm:pr-5'>
              {t('Total Top-up')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {usersQuery.data.map((user) => (
            <TableRow key={`${user.username}-${user.created_at}`}>
              <TableCell className='pl-4 font-medium sm:pl-5'>
                {user.username}
              </TableCell>
              <TableCell>{formatTimestamp(user.created_at)}</TableCell>
              <TableCell className='pr-4 text-right font-medium tabular-nums sm:pr-5'>
                {formatCurrencyFromUSD(user.total_top_up, currencyOptions)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    )
  } else {
    detailsContent = (
      <Empty className='min-h-40 border-0'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <Users aria-hidden='true' />
          </EmptyMedia>
          <EmptyTitle>{t('No referrals at this level')}</EmptyTitle>
          <EmptyDescription>
            {t('Referral details will appear here after users register.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <Collapsible open={open} onOpenChange={setOpen}>
      <Card className='gap-0 overflow-hidden py-0'>
        <CollapsibleTrigger className='hover:bg-muted/40 focus-visible:ring-ring flex w-full cursor-pointer items-center gap-3 px-4 py-4 text-left transition-colors focus-visible:ring-2 focus-visible:outline-none sm:px-5'>
          <div className='bg-primary/10 text-primary flex size-9 shrink-0 items-center justify-center rounded-lg'>
            <Users className='size-4' aria-hidden='true' />
          </div>
          <div className='min-w-0 flex-1'>
            <p className='font-medium'>
              {t('Level {{level}} Referrals', {
                level: props.summary.level,
              })}
            </p>
            <p className='text-muted-foreground mt-0.5 text-xs'>
              {t('{{count}} referrals', { count: props.summary.count })}
            </p>
          </div>
          <div className='shrink-0 text-right'>
            <p className='text-muted-foreground text-xs'>{t('Total Top-up')}</p>
            <p className='text-sm font-semibold tabular-nums'>
              {formatCurrencyFromUSD(
                props.summary.total_top_up,
                currencyOptions
              )}
            </p>
          </div>
          <ChevronDown
            className={`text-muted-foreground size-4 shrink-0 transition-transform ${open ? 'rotate-180' : ''}`}
            aria-hidden='true'
          />
        </CollapsibleTrigger>

        <CollapsibleContent className='CollapsibleContent border-t'>
          {detailsContent}
        </CollapsibleContent>
      </Card>
    </Collapsible>
  )
}
