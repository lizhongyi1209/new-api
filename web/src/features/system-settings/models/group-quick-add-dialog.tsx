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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

type Override = { targetGroup: string; ratio: number }
type RegistryEntry = { name: string; ratio: number }

type QuickAddOverrideDialogProps = {
  userGroup: string
  registry: RegistryEntry[]
  selectableGroupNames: string[]
  existingOverrides: Override[]
  onSave: (userGroup: string, overrides: Override[]) => void
  onClose: () => void
}

function roundGroupRatio(value: number): number {
  return Math.round(value * 1e6) / 1e6
}

export function QuickAddOverrideDialog(props: QuickAddOverrideDialogProps) {
  const { t } = useTranslation()
  const baseRatios = new Map(
    props.registry.map((entry) => [entry.name, entry.ratio])
  )
  const existingRatios = new Map(
    props.existingOverrides.map((entry) => [entry.targetGroup, entry.ratio])
  )
  const [multiplier, setMultiplier] = useState(() =>
    String(roundGroupRatio(baseRatios.get(props.userGroup) ?? 1))
  )
  const [selected, setSelected] = useState<Set<string>>(
    () =>
      new Set(
        props.selectableGroupNames.filter((name) => !existingRatios.has(name))
      )
  )

  const parsedMultiplier = Number(multiplier)
  const multiplierValid =
    multiplier.trim() !== '' &&
    Number.isFinite(parsedMultiplier) &&
    parsedMultiplier >= 0
  const rows = props.selectableGroupNames.map((name) => {
    const base = baseRatios.get(name) ?? 1
    return {
      name,
      base,
      final: multiplierValid
        ? roundGroupRatio(base * parsedMultiplier)
        : Number.NaN,
      existing: existingRatios.get(name),
    }
  })
  const selectedRows = rows.filter((row) => selected.has(row.name))
  const canApply =
    multiplierValid &&
    selectedRows.length > 0 &&
    selectedRows.every((row) => Number.isFinite(row.final) && row.final >= 0)
  const allSelected = rows.length > 0 && selectedRows.length === rows.length

  const handleApply = () => {
    if (!canApply) return
    props.onSave(
      props.userGroup,
      selectedRows.map((row) => ({ targetGroup: row.name, ratio: row.final }))
    )
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
      title={t('Quick add from selectable groups')}
      description={t(
        'Pick selectable groups for "{{userGroup}}". The final ratio is the group base ratio multiplied by the multiplier below.',
        { userGroup: props.userGroup }
      )}
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={props.onClose}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleApply} disabled={!canApply}>
            {t('Apply {{count}} ratios', { count: selectedRows.length })}
          </Button>
        </>
      }
    >
      <div className='space-y-2'>
        <Label htmlFor='group-quick-add-multiplier'>
          {t('Distribution multiplier')}
        </Label>
        <Input
          id='group-quick-add-multiplier'
          type='number'
          min={0}
          step='any'
          value={multiplier}
          onChange={(event) => setMultiplier(event.target.value)}
          aria-invalid={!multiplierValid}
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'Auto-filled from this user group base ratio. Final ratio = group base ratio x this multiplier.'
          )}
        </p>
      </div>

      {rows.length === 0 ? (
        <p className='text-muted-foreground text-sm'>
          {t(
            'No user-selectable groups configured. Add groups in the pricing table first.'
          )}
        </p>
      ) : (
        <StaticDataTable
          data={rows}
          getRowKey={(row) => row.name}
          columns={[
            {
              id: 'select',
              header: (
                <Checkbox
                  checked={allSelected}
                  onCheckedChange={(checked) =>
                    setSelected(
                      checked === true
                        ? new Set(rows.map((row) => row.name))
                        : new Set()
                    )
                  }
                  aria-label={t('Select all')}
                />
              ),
              className: 'w-10',
              cell: (row) => (
                <Checkbox
                  checked={selected.has(row.name)}
                  onCheckedChange={(checked) =>
                    setSelected((current) => {
                      const next = new Set(current)
                      if (checked === true) next.add(row.name)
                      else next.delete(row.name)
                      return next
                    })
                  }
                  aria-label={row.name}
                />
              ),
            },
            {
              id: 'group',
              header: t('Target group'),
              cellClassName: 'font-medium',
              cell: (row) => row.name,
            },
            {
              id: 'base',
              header: t('Base ratio'),
              cell: (row) => row.base,
            },
            {
              id: 'final',
              header: t('Final ratio'),
              cellClassName: 'font-semibold',
              cell: (row) => (Number.isFinite(row.final) ? row.final : '—'),
            },
            {
              id: 'existing',
              header: t('Current'),
              cell: (row) => row.existing ?? '—',
            },
          ]}
        />
      )}
    </Dialog>
  )
}
