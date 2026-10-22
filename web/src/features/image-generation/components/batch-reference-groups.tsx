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

import { Plus, Trash2 } from 'lucide-react'
import { nanoid } from 'nanoid'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { MAX_BATCH_GROUPS } from '../lib/batch-references'
import type {
  BatchCombinationMode,
  BatchReferenceGroup,
  ReferenceImage,
} from '../types'
import { ReferenceImagePicker } from './reference-image-picker'

type BatchReferenceGroupsProps = {
  disabled: boolean
  fixed: ReferenceImage[]
  groups: BatchReferenceGroup[]
  mode: BatchCombinationMode
  onFixedChange: (images: ReferenceImage[]) => void
  onGroupsChange: (groups: BatchReferenceGroup[]) => void
  onModeChange: (mode: BatchCombinationMode) => void
}

export function BatchReferenceGroups(props: BatchReferenceGroupsProps) {
  const { t } = useTranslation()
  const modeOptions = [
    { label: t('All combinations'), value: 'all' },
    { label: t('Pair by position'), value: 'paired' },
  ]

  return (
    <div className='space-y-4'>
      <ReferenceImagePicker
        disabled={props.disabled}
        images={props.fixed}
        label={t('Fixed reference images')}
        onChange={props.onFixedChange}
      />

      <div className='space-y-3'>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <p className='text-sm font-medium'>{t('Batch groups')}</p>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={props.disabled || props.groups.length >= MAX_BATCH_GROUPS}
            onClick={() =>
              props.onGroupsChange([
                ...props.groups,
                {
                  id: nanoid(),
                  name: t('Group {{number}}', {
                    number: props.groups.length + 1,
                  }),
                  images: [],
                },
              ])
            }
          >
            <Plus aria-hidden='true' />
            {t('Add group')}
          </Button>
        </div>

        <Select
          modal={false}
          items={modeOptions}
          value={props.mode}
          onValueChange={(value) =>
            value && props.onModeChange(value as BatchCombinationMode)
          }
          disabled={props.disabled}
        >
          <SelectTrigger
            aria-label={t('Combination mode')}
            className='w-full transition-none'
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false} animated={false}>
            {modeOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {props.groups.map((group, index) => (
          <div
            key={group.id}
            className='bg-muted/20 min-w-0 space-y-3 rounded-xl border p-3'
          >
            <div className='flex items-center gap-2'>
              <Input
                value={group.name}
                disabled={props.disabled}
                aria-label={t('Group {{number}} name', { number: index + 1 })}
                maxLength={40}
                onChange={(event) =>
                  props.onGroupsChange(
                    props.groups.map((item) =>
                      item.id === group.id
                        ? { ...item, name: event.target.value }
                        : item
                    )
                  )
                }
              />
              <Button
                type='button'
                size='icon-sm'
                variant='ghost'
                aria-label={t('Remove group {{number}}', {
                  number: index + 1,
                })}
                disabled={props.disabled || props.groups.length === 1}
                onClick={() =>
                  props.onGroupsChange(
                    props.groups.filter((item) => item.id !== group.id)
                  )
                }
              >
                <Trash2 aria-hidden='true' />
              </Button>
            </div>
            <ReferenceImagePicker
              disabled={props.disabled}
              images={group.images}
              label={t('{{name}} alternatives', {
                name:
                  group.name || t('Group {{number}}', { number: index + 1 }),
              })}
              onChange={(images) =>
                props.onGroupsChange(
                  props.groups.map((item) =>
                    item.id === group.id ? { ...item, images } : item
                  )
                )
              }
            />
          </div>
        ))}
      </div>
    </div>
  )
}
