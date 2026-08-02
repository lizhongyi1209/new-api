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
import { useTranslation } from 'react-i18next'

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'

interface TaskDetailDialogProps {
  record: Record<string, unknown>
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function TaskDetailDialog({
  record,
  open,
  onOpenChange,
}: TaskDetailDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Task Detail')}</DialogTitle>
          <DialogDescription>
            {t('Full request and response data for this task')}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='max-h-[600px]'>
          <div className='py-2'>
            <pre className='bg-muted text-muted-foreground overflow-x-auto rounded-md p-4 font-mono text-xs break-all whitespace-pre-wrap'>
              {JSON.stringify(record, null, 2)}
            </pre>
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
