import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { useTranslation } from 'react-i18next'

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
            <pre className='bg-muted text-muted-foreground rounded-md p-4 font-mono text-xs whitespace-pre-wrap break-all overflow-x-auto'>
              {JSON.stringify(record, null, 2)}
            </pre>
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
