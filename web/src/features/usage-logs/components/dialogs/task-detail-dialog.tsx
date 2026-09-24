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
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { ErrorState } from '@/components/error-state'
import { StatusBadge } from '@/components/status-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { formatLogQuota, formatTimestampToDate } from '@/lib/format'

import { getTaskArtifacts, getTaskAuditDetails } from '../../api'
import { taskActionMapper, taskStatusMapper } from '../../lib/mappers'
import type { TaskLog } from '../../types'
import { DetailRow, DetailSection } from './log-detail-layout'

type TaskDetailDialogProps = {
  log: TaskLog
  isAdmin: boolean
  sensitiveVisible: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function TaskDetailDialog(props: TaskDetailDialogProps) {
  const { t } = useTranslation()
  const audit = useQuery({
    queryKey: ['usage-logs', 'task-audit', props.log.task_id, props.isAdmin],
    queryFn: ({ signal }) => getTaskAuditDetails(props.log.task_id, signal),
    enabled: props.open && props.isAdmin,
    staleTime: 0,
    retry: false,
    meta: { errorToast: false, errorRedirect: false },
  })
  const log = props.log
  const artifacts = useQuery({
    queryKey: ['usage-logs', 'task-artifacts', log.task_id],
    queryFn: ({ signal }) => getTaskArtifacts(log.task_id, signal),
    enabled: props.open && log.channel_type === 62 && log.status === 'SUCCESS',
    staleTime: 0,
    retry: false,
    meta: { errorToast: false, errorRedirect: false },
  })
  const startTime =
    log.start_time ?? (props.isAdmin ? audit.data?.start_time : undefined)

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Task Detail')}
      description={t('View task status, timing and billing details.')}
      contentHeight='min(65dvh, 650px)'
      bodyClassName='space-y-3'
    >
      <DetailSection label={t('Basic Information')}>
        <DetailRow
          label={t('Task ID')}
          mono
          value={
            <span className='inline-flex max-w-full items-center gap-1'>
              <span className='min-w-0 break-all'>{log.task_id}</span>
              <CopyButton value={log.task_id} size='sm' className='size-6' />
            </span>
          }
        />
        <DetailRow
          label={t('Platform')}
          value={log.channel_type_name || log.platform}
        />
        <DetailRow
          label={t('Action')}
          value={t(taskActionMapper.getLabel(log.action))}
        />
        <DetailRow
          label={t('Status')}
          value={
            <StatusBadge
              label={t(
                taskStatusMapper.getLabel(
                  log.status,
                  log.status || 'Submitting'
                )
              )}
              variant={taskStatusMapper.getVariant(log.status)}
              copyable={false}
            />
          }
        />
        <DetailRow label={t('Progress')} value={log.progress || '-'} />
        <DetailRow
          label={t('Original Model')}
          value={log.properties?.origin_model_name || '-'}
          mono
        />
        <DetailRow
          label={t('Actual Model')}
          value={log.properties?.upstream_model_name || '-'}
          mono
        />
        <DetailRow
          label={t('Submit Time')}
          value={
            log.submit_time
              ? formatTimestampToDate(log.submit_time, 'seconds')
              : '-'
          }
          mono
        />
        <DetailRow
          label={t('Start Time')}
          value={startTime ? formatTimestampToDate(startTime, 'seconds') : '-'}
          mono
        />
        <DetailRow
          label={t('Finish Time')}
          value={
            log.finish_time
              ? formatTimestampToDate(log.finish_time, 'seconds')
              : '-'
          }
          mono
        />
      </DetailSection>
      {log.fail_reason && !/^https?:\/\//i.test(log.fail_reason) && (
        <DetailSection label={t('Fail Reason')} variant='danger'>
          <p className='text-xs break-all whitespace-pre-wrap'>
            {log.fail_reason}
          </p>
        </DetailSection>
      )}
      {log.channel_type === 62 && log.status === 'SUCCESS' && (
        <DetailSection label={t('Artifacts')}>
          {artifacts.isPending && <Skeleton className='h-28 w-full' />}
          {artifacts.isError && (
            <ErrorState
              title={t('Failed to load artifacts')}
              onRetry={() => void artifacts.refetch()}
              className='min-h-32'
            />
          )}
          {artifacts.data?.artifacts.map((artifact) => (
            <div key={artifact.key} className='space-y-2 rounded-md border p-3'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <span className='text-sm font-medium break-all'>{artifact.key}</span>
                <Button size='sm' variant='outline' render={
                  <a
                    href={`${artifact.content_url}${artifact.content_url.includes('?') ? '&' : '?'}download=1`}
                    target='_blank'
                    rel='noopener noreferrer'
                    download={artifact.key}
                  />
                }>
                  {t('Download')}
                </Button>
              </div>
              {artifact.type === 'image' && (
                <img
                  src={artifact.content_url}
                  alt={artifact.key}
                  className='max-h-80 max-w-full rounded object-contain'
                  loading='lazy'
                />
              )}
              {artifact.type === 'video' && (
                <video
                  src={artifact.content_url}
                  controls
                  preload='metadata'
                  className='max-h-80 max-w-full rounded'
                />
              )}
              {artifact.type === 'audio' && (
                <audio src={artifact.content_url} controls preload='metadata' className='w-full' />
              )}
            </div>
          ))}
        </DetailSection>
      )}
      {props.isAdmin && (
        <DetailSection label={t('Admin Only')}>
          <DetailRow
            label={t('User')}
            value={
              props.sensitiveVisible
                ? log.username || String(log.user_id)
                : '••••'
            }
          />
          <DetailRow
            label={t('Channel')}
            value={props.sensitiveVisible ? `#${log.channel_id}` : '••••'}
            mono
          />
          <DetailRow label={t('Group')} value={log.group || '-'} />
          {log.quota !== undefined && (
            <DetailRow label={t('Quota')} value={formatLogQuota(log.quota)} />
          )}
          {audit.isPending && <Skeleton className='h-8 w-full' />}
          {audit.isError && (
            <ErrorState
              title={t('Failed to load task details')}
              onRetry={() => void audit.refetch()}
              className='min-h-32'
            />
          )}
          {audit.data && (
            <>
              {log.quota === undefined && (
                <DetailRow
                  label={t('Quota')}
                  value={formatLogQuota(audit.data.quota)}
                />
              )}
              <DetailRow
                label={t('Request ID')}
                value={audit.data.request_id || '-'}
                mono
              />
              <DetailRow
                label={t('Upstream Task ID')}
                value={audit.data.upstream_task_id || '-'}
                mono
              />
            </>
          )}
        </DetailSection>
      )}
    </Dialog>
  )
}
