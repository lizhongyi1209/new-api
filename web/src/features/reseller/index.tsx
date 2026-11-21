import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { logout } from '@/features/auth/api'
import { toIntlLocale } from '@/i18n/languages'
import { clearAuthenticatedClientState } from '@/lib/auth-session'
import { formatQuotaWithCurrency } from '@/lib/currency'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { ResellerAccess } from './access'
import {
  emptyLogFilters,
  resellerGet,
  type Bootstrap,
  type Period,
} from './api'
import { CustomerTable } from './customers'
import { ResellerLogs } from './logs'
import {
  beijingStamp,
  beijingToday,
  dayRange,
  estimatedProfit,
  rangeBounds,
  type Markup,
} from './period'
import { ProfitDialog } from './profit'
import { RangeControl } from './range-control'

import '@/styles/reseller.css'

export function ResellerDesk() {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const user = useAuthStore((state) => state.auth.user)
  const currency = useSystemConfigStore((state) => state.config.currency)
  const client = useQueryClient()
  const navigate = useNavigate()
  const [view, setView] = useState<'customers' | 'logs' | 'access'>('customers')
  const [today, setToday] = useState(beijingToday)
  const [followToday, setFollowToday] = useState(true)
  const [customRange, setCustomRange] = useState(() => dayRange(beijingToday()))
  const range = followToday ? dayRange(today) : customRange
  const bounds = rangeBounds(range)
  const [filters, setFilters] = useState(emptyLogFilters)
  const [markup, setMarkup] = useState<Markup>({ base: 1.2, overrides: {} })
  const [profitOpen, setProfitOpen] = useState(false)
  const [signingOut, setSigningOut] = useState(false)
  const bootstrap = useQuery({
    queryKey: ['reseller', 'bootstrap', user?.id],
    queryFn: ({ signal }) =>
      resellerGet<Bootstrap>('bootstrap', undefined, signal),
    refetchInterval: 60000,
    retry: false,
    meta: { errorToast: false },
  })
  const stats = useQuery({
    queryKey: ['reseller', 'period', user?.id, bounds],
    queryFn: ({ signal }) => resellerGet<Period>('period', bounds, signal),
    enabled: bootstrap.isSuccess,
    refetchInterval: 60000,
    retry: false,
    meta: { errorToast: false },
  })
  const period = stats.isError ? undefined : stats.data
  const isToday = range.from === today && range.to === today && !range.precise
  const titles = {
    customers: t('Direct customers'),
    logs: t('Usage logs'),
    access: t('Reseller access'),
  }
  const title = titles[view]
  useEffect(() => {
    document.title = `${title} · O1Key`
  }, [title])
  useEffect(() => {
    const refresh = () => setToday(beijingToday())
    const timer = setInterval(refresh, 30000)
    document.addEventListener('visibilitychange', refresh)
    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', refresh)
    }
  }, [])
  // Subscribe to configuration changes so quota formatting never uses stale units.
  const amount = (quota: number | null | undefined) =>
    currency &&
    formatQuotaWithCurrency(quota, {
      showSymbol: false,
      abbreviate: false,
      digitsLarge: 2,
      digitsSmall: 2,
      locale,
    })
  const profit = period
    ? estimatedProfit(period.spent_quota, period.pricing_groups, markup)
    : null

  async function signOut(): Promise<void> {
    setSigningOut(true)
    try {
      requireServerSuccess(await logout())
      clearAuthenticatedClientState(client)
      await navigate({ to: '/reseller/login', replace: true })
    } catch (error) {
      handleServerError(error, t('Failed to sign out session'))
    } finally {
      setSigningOut(false)
    }
  }

  return (
    <div className='reseller-ledger'>
      <header className='ledger-masthead'>
        <b className='ledger-brand'>O1Key</b>
        <div className='ledger-account'>
          <span>{user?.username}</span>
          <Button
            variant='link'
            disabled={signingOut}
            onClick={() => void signOut()}
          >
            {t('Sign out')}
          </Button>
        </div>
      </header>
      <nav className='ledger-nav' aria-label={t('Reseller center')}>
        <Button
          variant='ghost'
          aria-pressed={view === 'customers'}
          onClick={() => setView('customers')}
        >
          {t('Customer balances')}
        </Button>
        <Button
          variant='ghost'
          aria-pressed={view === 'logs'}
          onClick={() => setView('logs')}
        >
          {t('Usage logs')}
        </Button>
        {(user?.role ?? 0) >= 100 && (
          <Button
            variant='ghost'
            aria-pressed={view === 'access'}
            onClick={() => setView('access')}
          >
            {t('Reseller access')}
          </Button>
        )}
      </nav>
      <main className='ledger-main'>
        <div className='ledger-heading'>
          <div>
            <p className='ledger-eyebrow'>{t('Reseller center')}</p>
            <h1 className='ledger-title'>{title}</h1>
          </div>
          {view !== 'access' && (
            <div className='ledger-period-controls'>
              <RangeControl
                value={range}
                onChange={(value, follow) => {
                  setCustomRange(value)
                  setFollowToday(follow)
                }}
              />
              <span className='ledger-muted'>
                {period
                  ? t('Updated {{time}}', {
                      time: beijingStamp(period.as_of).slice(11, 16),
                    })
                  : ''}
              </span>
            </div>
          )}
        </div>
        {bootstrap.isPending && <LoadingState />}
        {bootstrap.isError && (
          <ErrorState
            title={t('Unable to open reseller center')}
            description={t('Use an account with reseller access.')}
            onRetry={() => void bootstrap.refetch()}
          />
        )}
        {bootstrap.isSuccess && view !== 'access' && (
          <>
            <dl className='ledger-stats' aria-label={t('All direct customers')}>
              <div>
                <dt>
                  {isToday ? t('Today consumption') : t('Period consumption')}
                </dt>
                <dd>{amount(period?.spent_quota)}</dd>
              </div>
              <div>
                <dt>{isToday ? t('Today recharge') : t('Period recharge')}</dt>
                <dd>{amount(period?.topup_quota)}</dd>
              </div>
              <div>
                <dt>{isToday ? t('Today refunds') : t('Period refunds')}</dt>
                <dd>
                  <Button
                    variant='link'
                    className='ledger-stat-button'
                    disabled={!period}
                    onClick={() => {
                      setFilters({ ...emptyLogFilters, type: '6' })
                      setView('logs')
                    }}
                  >
                    {amount(period?.refund_quota)}
                  </Button>
                </dd>
              </div>
              <div>
                <dt>
                  {period?.estimated_balance
                    ? t('Closing balance (est.)')
                    : t('Remaining balance')}
                </dt>
                <dd>{amount(period?.remaining_quota)}</dd>
              </div>
              <div>
                <dt>
                  {period?.estimated_balance
                    ? t('Closing usage (est.)')
                    : t('Used balance')}
                </dt>
                <dd>{amount(period?.used_quota)}</dd>
              </div>
              <div>
                <dt>{t('Estimated profit')}</dt>
                <dd className='ledger-profit'>
                  {amount(profit)}
                  <Button
                    variant='link'
                    disabled={!period}
                    onClick={() => setProfitOpen(true)}
                  >
                    {Object.keys(markup.overrides).length
                      ? t('Group estimate')
                      : t('{{rate}}× estimate', { rate: markup.base })}
                  </Button>
                </dd>
              </div>
            </dl>
            {stats.isError && (
              <p role='alert' className='ledger-error'>
                {t('Statistics unavailable')}{' '}
                <Button variant='link' onClick={() => void stats.refetch()}>
                  {t('Retry')}
                </Button>
              </p>
            )}
            {view === 'customers' && (
              <CustomerTable
                customers={bootstrap.data.customers}
                onLogs={(id) => {
                  setFilters({ ...emptyLogFilters, user: String(id) })
                  setView('logs')
                }}
              />
            )}
            {view === 'logs' && (
              <ResellerLogs
                key={`${bounds.from}:${bounds.to}:${filters.user}:${filters.type}`}
                bootstrap={bootstrap.data}
                bounds={bounds}
                filters={filters}
                onFilters={setFilters}
              />
            )}
          </>
        )}
        {bootstrap.isSuccess && view === 'access' && <ResellerAccess />}
      </main>
      {profitOpen && period && (
        <ProfitDialog
          key={`${bounds.from}:${bounds.to}`}
          period={period}
          value={markup}
          onChange={setMarkup}
          onClose={() => setProfitOpen(false)}
        />
      )}
    </div>
  )
}
