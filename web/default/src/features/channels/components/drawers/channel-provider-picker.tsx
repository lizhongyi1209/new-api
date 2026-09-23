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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  CHANNEL_TYPE_ADVANCED_CUSTOM,
  CHANNEL_TYPE_OPTIONS,
  CHANNEL_TYPE_TASK_PLUGIN,
} from '../../constants'
import { ChannelTypeLogo } from '../channel-type-logo'

type ChannelProviderPickerProps = {
  currentType: number
  canBindTaskPlugin: boolean
  disabled: boolean
  onSelect: (type: number) => void
}

export function ChannelProviderPicker(props: ChannelProviderPickerProps) {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const [filter, setFilter] = useState('all')
  const options = useMemo(() => {
    const keyword = search.trim().toLocaleLowerCase()
    const entries = CHANNEL_TYPE_OPTIONS.filter(
      (option) => option.value !== CHANNEL_TYPE_TASK_PLUGIN || props.canBindTaskPlugin
    ).map((option) => ({
      ...option,
      label: t(option.label),
      searchText: `${option.value} ${option.label} ${t(option.label)}`,
    }))
    if (
      props.currentType > 0 &&
      !entries.some((option) => option.value === props.currentType)
    ) {
      entries.push({
        value: props.currentType,
        label: `#${props.currentType}`,
        searchText: String(props.currentType),
      })
    }
    return entries.filter((option) => {
      const custom =
        option.value === 8 || option.value === CHANNEL_TYPE_ADVANCED_CUSTOM
      const gateway = option.value === 63 || option.value === 64
      if (filter === 'custom' && !custom) return false
      if (filter === 'gateway' && !gateway) return false
      if (filter === 'builtin' && (custom || gateway)) return false
      return option.searchText.toLocaleLowerCase().includes(keyword)
    })
  }, [filter, props.canBindTaskPlugin, props.currentType, search, t])
  const customType = Number(search.trim())
  const canUseCustomType =
    (filter === 'all' || filter === 'custom') &&
    /^\d+$/.test(search.trim()) &&
    Number.isSafeInteger(customType) &&
    customType > 0 &&
    !options.some((option) => option.value === customType) &&
    !CHANNEL_TYPE_OPTIONS.some((option) => option.value === customType)

  return (
    <Tabs
      value={filter}
      onValueChange={(value) => setFilter(String(value))}
      className='min-h-0 flex-1 gap-4'
    >
      <TabsList
        aria-label={t('Provider source')}
        className='max-w-full shrink-0 flex-wrap justify-start group-data-horizontal/tabs:h-auto'
      >
        <TabsTrigger value='all' className='h-auto'>
          {t('All')}
        </TabsTrigger>
        <TabsTrigger value='builtin' className='h-auto'>
          {t('Built-in')}
        </TabsTrigger>
        <TabsTrigger value='gateway' className='h-auto'>
          {t('Gateways')}
        </TabsTrigger>
        <TabsTrigger value='custom' className='h-auto'>
          {t('Custom')}
        </TabsTrigger>
      </TabsList>
      <TabsContent
        value={filter}
        keepMounted
        className='flex min-h-0 flex-1 flex-col'
      >
        <Command
          label={t('Search providers or type numbers')}
          defaultValue={String(props.currentType)}
          shouldFilter={false}
          className='min-h-0 flex-1 bg-transparent p-0'
        >
          <CommandInput
            autoFocus
            value={search}
            onValueChange={setSearch}
            placeholder={t('Search providers or type numbers')}
            aria-label={t('Search providers or type numbers')}
            disabled={props.disabled}
          />
          <CommandList className='mt-3 max-h-none min-h-0 flex-1'>
            <CommandEmpty>{t('No channel type found.')}</CommandEmpty>
            <CommandGroup className='p-1 [&_[cmdk-group-items]]:grid [&_[cmdk-group-items]]:gap-2.5 sm:[&_[cmdk-group-items]]:grid-cols-2 lg:[&_[cmdk-group-items]]:grid-cols-3'>
              {options.map((option) => (
                <CommandItem
                  key={option.value}
                  value={String(option.value)}
                  aria-label={`${option.label} #${option.value}`}
                  aria-current={
                    option.value === props.currentType ? true : undefined
                  }
                  disabled={props.disabled}
                  onSelect={() => props.onSelect(option.value)}
                  className='data-selected:border-primary/50 data-selected:bg-primary/5 min-h-20 gap-3 rounded-lg border p-3 [&>svg]:hidden'
                >
                  <ChannelTypeLogo type={option.value} size={22} />
                  <span
                    className='min-w-0 flex-1 truncate'
                    title={option.label}
                  >
                    {option.label}
                  </span>
                  <Badge variant='outline'>#{option.value}</Badge>
                </CommandItem>
              ))}
              {canUseCustomType && (
                <CommandItem
                  value={String(customType)}
                  aria-label={`#${customType}`}
                  disabled={props.disabled}
                  onSelect={() => props.onSelect(customType)}
                >
                  <ChannelTypeLogo type={customType} />#{customType}
                </CommandItem>
              )}
            </CommandGroup>
          </CommandList>
        </Command>
      </TabsContent>
    </Tabs>
  )
}
