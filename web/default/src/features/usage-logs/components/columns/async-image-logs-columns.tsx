/* eslint-disable react-refresh/only-export-components */
import { useState } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { Image } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/status-badge'
import { TASK_STATUS } from '../../constants'
import {
  asyncImageActionMapper,
  asyncImagePlatformMapper,
  taskStatusMapper,
} from '../../lib/mappers'
import type { TaskLog } from '../../types'
import { ImageDialog } from '../dialogs/image-dialog'
import { FailReasonDialog } from '../dialogs/fail-reason-dialog'
import {
  createTimestampColumn,
  createDurationColumn,
  createChannelColumn,
  createProgressColumn,
} from './column-helpers'

export function useAsyncImageLogsColumns(
  isAdmin: boolean
): ColumnDef<TaskLog>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<TaskLog>[] = [
    createTimestampColumn<TaskLog>({
      accessorKey: 'submit_time',
      title: t('Submit Time'),
      unit: 'seconds',
    }),
    createTimestampColumn<TaskLog>({
      accessorKey: 'finish_time',
      title: t('Finish Time'),
      unit: 'seconds',
    }),
    createDurationColumn<TaskLog>({
      submitTimeKey: 'submit_time',
      finishTimeKey: 'finish_time',
      unit: 'seconds',
      headerLabel: t('Duration'),
    }),
  ]

  // Channel (admin only)
  if (isAdmin) {
    columns.push(createChannelColumn<TaskLog>({ headerLabel: t('Channel') }))
  }

  columns.push(
    // Platform
    {
      accessorKey: 'platform',
      header: t('Platform'),
      cell: ({ row }) => {
        const platform = row.getValue('platform') as string
        return (
          <StatusBadge
            label={t(asyncImagePlatformMapper.getLabel(platform, platform))}
            variant={asyncImagePlatformMapper.getVariant(platform)}
            size='sm'
            copyable={false}
          />
        )
      },
      meta: { label: t('Platform') },
    },

    // Type/Action
    {
      accessorKey: 'action',
      header: t('Type'),
      cell: ({ row }) => {
        const action = row.getValue('action') as string
        return (
          <StatusBadge
            label={t(asyncImageActionMapper.getLabel(action, action))}
            variant={asyncImageActionMapper.getVariant(action)}
            size='sm'
            copyable={false}
          />
        )
      },
      meta: { label: t('Type') },
    },

    // Task ID
    {
      accessorKey: 'task_id',
      header: t('Task ID'),
      cell: ({ row }) => {
        const taskId = row.getValue('task_id') as string
        return (
          <StatusBadge
            label={taskId}
            autoColor={taskId}
            size='sm'
            className='font-mono'
          />
        )
      },
      meta: { label: t('Task ID'), mobileHidden: true },
    },

    // Status
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        return (
          <StatusBadge
            label={t(taskStatusMapper.getLabel(status, status || 'Submitting'))}
            variant={taskStatusMapper.getVariant(status)}
            size='sm'
            copyable={false}
            showDot
          />
        )
      },
      meta: { label: t('Status') },
    },

    createProgressColumn<TaskLog>({ headerLabel: t('Progress') }),

    // Result/Details
    {
      accessorKey: 'fail_reason',
      header: t('Details'),
      cell: function DetailsCell({ row }) {
        const log = row.original
        const failReason = row.getValue('fail_reason') as string
        const resultUrl = log.result_url
        const status = log.status
        const [dialogOpen, setDialogOpen] = useState(false)

        // Successful async image with result_url
        if (status === TASK_STATUS.SUCCESS && resultUrl) {
          return (
            <>
              <Button
                variant='ghost'
                className='h-auto p-0 text-left text-sm font-normal text-primary hover:underline'
                onClick={() => setDialogOpen(true)}
              >
                <Image className='mr-1 h-3 w-3' />
                {t('Click to view image')}
              </Button>
              <ImageDialog
                imageUrl={resultUrl}
                taskId={log.task_id}
                open={dialogOpen}
                onOpenChange={setDialogOpen}
              />
            </>
          )
        }

        // Error or no result
        if (!failReason) {
          return <span className='text-muted-foreground text-sm'>-</span>
        }

        return (
          <>
            <Button
              variant='ghost'
              className='h-auto max-w-[200px] justify-start overflow-hidden p-0 text-left text-sm font-normal text-red-600 hover:underline'
              onClick={() => setDialogOpen(true)}
              title={t('Click to view full error message')}
            >
              <span className='truncate'>{failReason}</span>
            </Button>
            <FailReasonDialog
              failReason={failReason}
              open={dialogOpen}
              onOpenChange={setDialogOpen}
            />
          </>
        )
      },
      meta: { label: t('Details') },
      size: 200,
      maxSize: 220,
    }
  )

  return columns
}
