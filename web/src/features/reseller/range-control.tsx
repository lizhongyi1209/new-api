import { zodResolver } from '@hookform/resolvers/zod'
import { CalendarDays } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { beijingToday, dayRange, rangeBounds, type DateRange } from './period'

const rangeSchema = z
  .object({
    from: z.string().min(1),
    to: z.string().min(1),
    start: z.string().min(1),
    end: z.string().min(1),
    precise: z.boolean(),
  })
  .refine((value) => {
    const bounds = rangeBounds(value)
    return (
      Number.isFinite(bounds.from) &&
      Number.isFinite(bounds.to) &&
      bounds.from <= bounds.to &&
      value.to <= beijingToday()
    )
  })

export function RangeControl(props: {
  value: DateRange
  onChange: (value: DateRange, followToday: boolean) => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const form = useForm<DateRange>({
    resolver: zodResolver(rangeSchema),
    defaultValues: props.value,
  })
  const precise = form.watch('precise')
  const today = beijingToday()
  let label = `${props.value.from} — ${props.value.to}`
  if (props.value.from === props.value.to) {
    label = props.value.from === today ? t('Today') : props.value.from
  }
  if (props.value.precise) label += ` ${props.value.start}–${props.value.end}`

  function apply(value: DateRange): void {
    props.onChange(
      value,
      !value.precise && value.from === today && value.to === today
    )
    setOpen(false)
  }

  function quickRange(key: string): void {
    const date = new Date(`${today}T00:00:00+08:00`)
    let from = today
    let to = today
    if (key === 'yesterday' || key === 'week') {
      date.setTime(date.getTime() - (key === 'yesterday' ? 1 : 6) * 86400000)
      from = new Intl.DateTimeFormat('sv-SE', {
        timeZone: 'Asia/Shanghai',
      }).format(date)
      if (key === 'yesterday') to = from
    }
    if (key === 'month') from = `${today.slice(0, 7)}-01`
    apply(dayRange(from, to))
  }

  return (
    <>
      <Button
        variant='outline'
        className='ledger-range'
        onClick={() => {
          form.reset(props.value)
          setOpen(true)
        }}
      >
        <CalendarDays aria-hidden='true' />
        {label}
      </Button>
      <Dialog
        open={open}
        onOpenChange={setOpen}
        title={t('Date range')}
        contentClassName='ledger-dialog sm:max-w-lg'
      >
        <form onSubmit={form.handleSubmit(apply)} className='space-y-5'>
          <div className='flex flex-wrap gap-2'>
            {(
              [
                ['today', 'Today'],
                ['yesterday', 'Yesterday'],
                ['week', 'Last 7 days'],
                ['month', 'This month'],
              ] as const
            ).map(([key, text]) => (
              <Button
                key={key}
                type='button'
                variant='outline'
                onClick={() => quickRange(key)}
              >
                {t(text)}
              </Button>
            ))}
          </div>
          <div className='grid grid-cols-2 gap-4'>
            <div className='space-y-2'>
              <Label htmlFor='ledger-start'>{t('Start date')}</Label>
              <Input
                id='ledger-start'
                type='date'
                max={today}
                {...form.register('from')}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='ledger-end'>{t('End date')}</Label>
              <Input
                id='ledger-end'
                type='date'
                max={today}
                {...form.register('to')}
              />
            </div>
          </div>
          <Label className='flex items-center gap-2'>
            <Checkbox
              checked={precise}
              onCheckedChange={(checked) =>
                form.setValue('precise', Boolean(checked))
              }
            />
            {t('Exact time')}
          </Label>
          {precise && (
            <div className='grid grid-cols-2 gap-4'>
              <div className='space-y-2'>
                <Label htmlFor='ledger-start-time'>{t('Start time')}</Label>
                <Input
                  id='ledger-start-time'
                  type='time'
                  step='1'
                  {...form.register('start')}
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='ledger-end-time'>{t('End time')}</Label>
                <Input
                  id='ledger-end-time'
                  type='time'
                  step='1'
                  {...form.register('end')}
                />
              </div>
            </div>
          )}
          <p className='ledger-muted'>{t('Beijing time')}</p>
          {Object.keys(form.formState.errors).length > 0 && (
            <p role='alert' className='text-red-700'>
              {t('Invalid date range')}
            </p>
          )}
          <div className='flex justify-end gap-2'>
            <Button
              type='button'
              variant='outline'
              onClick={() => setOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button type='submit'>{t('Apply')}</Button>
          </div>
        </form>
      </Dialog>
    </>
  )
}
