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
import type { Row } from '@tanstack/react-table'
import {
  RefreshCw,
  Trash2,
  Copy,
  AtSign,
  MoreHorizontal as DotsHorizontalIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { refreshAigcElement } from '../api'
import { SUCCESS_MESSAGES } from '../constants'
import { aigcElementSchema } from '../types'
import { useAigcElements } from './aigc-elements-provider'

interface DataTableRowActionsProps<TData> {
  row: Row<TData>
}

export function DataTableRowActions<TData>({
  row,
}: DataTableRowActionsProps<TData>) {
  const { t } = useTranslation()
  const element = aigcElementSchema.parse(row.original)
  const { setOpen, setCurrentRow, triggerRefresh } = useAigcElements()
  const { copyToClipboard } = useCopyToClipboard({ notify: false })
  const [isRefreshing, setIsRefreshing] = useState(false)

  const handleRefresh = async () => {
    if (!element.element_id) {
      toast.error(t('This subject has no Element ID yet'))
      return
    }
    setIsRefreshing(true)
    try {
      const result = await refreshAigcElement(element.id)
      if (result.success) {
        toast.success(t(SUCCESS_MESSAGES.ELEMENT_REFRESHED))
        triggerRefresh()
      }
    } finally {
      setIsRefreshing(false)
    }
  }

  // Copy the 【@Name】 tag used to reference this subject inside a video prompt.
  const handleCopyTag = async () => {
    const ok = await copyToClipboard(`【@${element.name}】`)
    if (ok) {toast.success(t('Prompt tag copied'))}
  }

  const handleCopyElementId = async () => {
    if (!element.element_id) {
      toast.error(t('This subject has no Element ID yet'))
      return
    }
    const ok = await copyToClipboard(element.element_id)
    if (ok) {toast.success(t('Element ID copied'))}
  }

  return (
    <div className='-ml-2'>
      <DropdownMenu modal={false}>
        <DropdownMenuTrigger
          render={
            <Button
              variant='ghost'
              className='data-popup-open:bg-muted flex h-8 w-8 p-0'
            />
          }
        >
          <DotsHorizontalIcon className='h-4 w-4' />
          <span className='sr-only'>{t('Open menu')}</span>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-[180px]'>
          <DropdownMenuItem onClick={handleCopyTag}>
            {t('Copy @Name')}
            <DropdownMenuShortcut>
              <AtSign size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuItem onClick={handleCopyElementId}>
            {t('Copy Element ID')}
            <DropdownMenuShortcut>
              <Copy size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem onClick={handleRefresh} disabled={isRefreshing}>
            {t('Refresh Status')}
            <DropdownMenuShortcut>
              <RefreshCw size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(element)
              setOpen('delete')
            }}
            className='text-destructive focus:text-destructive'
          >
            {t('Delete')}
            <DropdownMenuShortcut>
              <Trash2 size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
