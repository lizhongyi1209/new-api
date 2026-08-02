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
import { DownloadIcon, Loader2Icon, RefreshCcwIcon } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { ComboboxInput } from '@/components/ui/combobox-input'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useDebounce } from '@/hooks'

import { exportUsageLogs, getUsageLogExportOptions } from '../api'
import { buildApiParams, getDefaultTimeRange } from '../lib/utils'
import type { UsageLogExportFormat } from '../types'
import { CompactDateTimeRangePicker } from './compact-date-time-range-picker'

const refreshOptions = [
  { value: '0', label: 'Off' },
  { value: '5000', label: '5s' },
  { value: '10000', label: '10s' },
  { value: '30000', label: '30s' },
  { value: '60000', label: '1m' },
  { value: '300000', label: '5m' },
] as const

const exportFormats: Array<{
  value: UsageLogExportFormat
  label: string
  description: string
}> = [
  {
    value: 'csv',
    label: 'CSV',
    description: 'Best for spreadsheets and quick analysis',
  },
  {
    value: 'json',
    label: 'JSON',
    description: 'Complete structured log records',
  },
  {
    value: 'xlsx',
    label: 'Reconciliation XLSX',
    description: 'Formatted statement for billing reconciliation',
  },
]

interface UsageLogsActionsProps {
  autoRefreshMs: number
  onAutoRefreshChange: (interval: number) => void
  isAdmin: boolean
  showExport: boolean
  searchParams: Record<string, unknown>
}

export function UsageLogsActions(props: UsageLogsActionsProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [format, setFormat] = useState<UsageLogExportFormat>('csv')
  const [model, setModel] = useState('')
  const [group, setGroup] = useState('')
  const [username, setUsername] = useState('')
  const [token, setToken] = useState('')
  const debouncedUsername = useDebounce(username)
  const defaultRange = useMemo(() => getDefaultTimeRange(), [])
  const [start, setStart] = useState(defaultRange.start)
  const [end, setEnd] = useState(defaultRange.end)

  useEffect(() => {
    if (!open) return
    const range = getDefaultTimeRange()
    setStart(
      props.searchParams.startTime
        ? new Date(Number(props.searchParams.startTime))
        : range.start
    )
    setEnd(
      props.searchParams.endTime
        ? new Date(Number(props.searchParams.endTime))
        : range.end
    )
    setModel(String(props.searchParams.model || ''))
    setGroup(String(props.searchParams.group || ''))
    setUsername(String(props.searchParams.username || ''))
    setToken(String(props.searchParams.token || ''))
  }, [open, props.searchParams])

  const exportOptionsQuery = useQuery({
    queryKey: [
      'usage-log-export-options',
      start.getTime(),
      end.getTime(),
      debouncedUsername,
    ],
    queryFn: () =>
      getUsageLogExportOptions({
        start_timestamp: Math.floor(start.getTime() / 1000),
        end_timestamp: Math.floor(end.getTime() / 1000),
        ...(debouncedUsername ? { username: debouncedUsername } : {}),
      }),
    enabled:
      open &&
      props.isAdmin &&
      start.getTime() > 0 &&
      end.getTime() >= start.getTime() &&
      end.getTime() - start.getTime() <= 31 * 24 * 60 * 60 * 1000,
    staleTime: 30_000,
  })
  const usernameOptions = useMemo(
    () =>
      (exportOptionsQuery.data?.usernames || []).map((value) => ({
        value,
        label: value,
      })),
    [exportOptionsQuery.data?.usernames]
  )
  const tokenOptions = useMemo(
    () =>
      (exportOptionsQuery.data?.tokens || []).map((value) => ({
        value,
        label: value,
      })),
    [exportOptionsQuery.data?.tokens]
  )

  const refreshItems = useMemo(
    () =>
      refreshOptions.map((option) => ({
        value: option.value,
        label: option.value === '0' ? t(option.label) : option.label,
      })),
    [t]
  )
  const activeRefreshLabel =
    refreshItems.find((option) => option.value === String(props.autoRefreshMs))
      ?.label || t('Off')

  const handleExport = useCallback(async () => {
    if (end.getTime() < start.getTime()) {
      toast.error(t('End time must be after start time'))
      return
    }
    if (end.getTime() - start.getTime() > 31 * 24 * 60 * 60 * 1000) {
      toast.error(t('Export time range cannot exceed 31 days'))
      return
    }

    setExporting(true)
    try {
      const baseParams = buildApiParams({
        page: 1,
        pageSize: 1,
        searchParams: {
          startTime: start.getTime(),
          endTime: end.getTime(),
          model,
          group,
          username,
          token,
        },
        isAdmin: props.isAdmin,
      })
      delete baseParams.p
      delete baseParams.page_size
      const result = await exportUsageLogs(
        { ...baseParams, format },
        props.isAdmin
      )
      const url = URL.createObjectURL(result.blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = result.filename
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      URL.revokeObjectURL(url)
      toast.success(t('Log export downloaded'))
      setOpen(false)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to export logs')
      )
    } finally {
      setExporting(false)
    }
  }, [end, format, group, model, props.isAdmin, start, t, token, username])

  return (
    <div className='flex items-center gap-1'>
      <div className='bg-background/70 flex items-center rounded-lg border'>
        <RefreshCcwIcon className='text-muted-foreground ml-2 size-3.5' />
        <Select
          items={refreshItems}
          value={String(props.autoRefreshMs)}
          onValueChange={(value) =>
            props.onAutoRefreshChange(Number(value || 0))
          }
        >
          <SelectTrigger
            size='sm'
            className='h-7 w-[4.75rem] border-0 bg-transparent shadow-none'
            aria-label={t('Auto refresh')}
          >
            <SelectValue>{activeRefreshLabel}</SelectValue>
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {refreshItems.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>

      {props.showExport && (
        <Sheet open={open} onOpenChange={setOpen}>
          <Tooltip>
            <TooltipTrigger
              render={
                <SheetTrigger
                  render={
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='text-muted-foreground hover:text-foreground size-7'
                      aria-label={t('Export logs')}
                    />
                  }
                />
              }
            >
              <DownloadIcon className='size-4' />
            </TooltipTrigger>
            <TooltipContent>{t('Export logs')}</TooltipContent>
          </Tooltip>

          <SheetContent side='right' className='w-full sm:max-w-md'>
            <SheetHeader className='border-b px-5 py-4'>
              <SheetTitle>{t('Export usage logs')}</SheetTitle>
              <SheetDescription>
                {t(
                  'Export filtered records for analysis or billing reconciliation.'
                )}
              </SheetDescription>
            </SheetHeader>

            <div className='flex-1 space-y-6 overflow-y-auto px-5 py-2'>
              <div className='space-y-2'>
                <Label>{t('Time Range')}</Label>
                <CompactDateTimeRangePicker
                  start={start}
                  end={end}
                  onChange={(range) => {
                    if (range.start) setStart(range.start)
                    if (range.end) setEnd(range.end)
                  }}
                />
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'A single export can cover up to 31 days and 50,000 records.'
                  )}
                </p>
              </div>

              <div className='grid gap-4 sm:grid-cols-2'>
                <div className='space-y-2'>
                  <Label htmlFor='usage-log-export-group'>{t('Group')}</Label>
                  <Input
                    id='usage-log-export-group'
                    value={group}
                    onChange={(event) => setGroup(event.target.value)}
                    placeholder={t('All groups')}
                  />
                </div>
                <div className='space-y-2'>
                  <Label htmlFor='usage-log-export-model'>
                    {t('Model Name')}
                  </Label>
                  <Input
                    id='usage-log-export-model'
                    value={model}
                    onChange={(event) => setModel(event.target.value)}
                    placeholder={t('All models')}
                  />
                </div>
              </div>

              {props.isAdmin && (
                <div className='grid gap-4 sm:grid-cols-2'>
                  <div className='space-y-2'>
                    <Label htmlFor='usage-log-export-username'>
                      {t('Username')}
                    </Label>
                    <ComboboxInput
                      id='usage-log-export-username'
                      options={usernameOptions}
                      value={username}
                      onValueChange={(value) => {
                        setUsername(value)
                        setToken('')
                      }}
                      placeholder={t('All users')}
                      emptyText={t('No data')}
                      allowCustomValue
                    />
                  </div>
                  <div className='space-y-2'>
                    <Label htmlFor='usage-log-export-token'>
                      {t('Token Name')}
                    </Label>
                    <ComboboxInput
                      id='usage-log-export-token'
                      options={tokenOptions}
                      value={token}
                      onValueChange={setToken}
                      placeholder={t('Token Name')}
                      emptyText={t('No data')}
                      allowCustomValue
                    />
                  </div>
                </div>
              )}

              <div className='space-y-3'>
                <Label>{t('Export Format')}</Label>
                <RadioGroup
                  value={format}
                  onValueChange={(value) =>
                    setFormat(value as UsageLogExportFormat)
                  }
                >
                  {exportFormats.map((option) => (
                    <Label
                      key={option.value}
                      className='hover:bg-muted/50 flex cursor-pointer items-start gap-3 rounded-xl border p-3 transition-colors'
                    >
                      <RadioGroupItem value={option.value} className='mt-0.5' />
                      <span className='space-y-1'>
                        <span className='block font-medium'>
                          {t(option.label)}
                        </span>
                        <span className='text-muted-foreground block text-xs leading-4 font-normal'>
                          {t(option.description)}
                        </span>
                      </span>
                    </Label>
                  ))}
                </RadioGroup>
              </div>
            </div>

            <SheetFooter className='border-t px-5 py-4'>
              <Button onClick={handleExport} disabled={exporting}>
                {exporting ? (
                  <Loader2Icon className='size-4 animate-spin' />
                ) : (
                  <DownloadIcon className='size-4' />
                )}
                {exporting ? t('Exporting...') : t('Export')}
              </Button>
            </SheetFooter>
          </SheetContent>
        </Sheet>
      )}
    </div>
  )
}
