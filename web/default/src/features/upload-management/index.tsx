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
import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import { FileImage, FolderOpen, RefreshCw, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { toIntlLocale } from '@/i18n/languages'

import {
  batchDeleteFiles,
  clearUploadedFiles,
  deleteFile,
  getUploadedFiles,
  getUploadStats,
  type FileInfo,
  type StorageStats,
  type UploadCategory,
} from './api'

const PAGE_SIZE = 50

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes / 1024
  let unitIndex = 0
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024
    unitIndex++
  }
  return `${value.toFixed(value >= 10 ? 1 : 2)} ${units[unitIndex]}`
}

function StatCard(props: {
  title: string
  value?: { count: number; size: number }
}) {
  return (
    <Card>
      <CardHeader className='pb-2'>
        <CardTitle className='text-muted-foreground text-sm font-medium'>
          {props.title}
        </CardTitle>
      </CardHeader>
      <CardContent>
        {props.value ? (
          <div className='flex items-end justify-between gap-3'>
            <span className='text-2xl font-semibold tabular-nums'>
              {props.value.count}
            </span>
            <span className='text-muted-foreground text-xs tabular-nums'>
              {formatFileSize(props.value.size)}
            </span>
          </div>
        ) : (
          <Skeleton className='h-8 w-full' />
        )}
      </CardContent>
    </Card>
  )
}

function FilePreview(props: { file: FileInfo; fileLabel: string }) {
  if (!props.file.is_image) {
    return (
      <div className='bg-muted text-muted-foreground flex size-12 items-center justify-center rounded-md'>
        <FileImage className='size-5' aria-hidden='true' />
        <span className='sr-only'>{props.fileLabel}</span>
      </div>
    )
  }
  return (
    <img
      src={props.file.thumbnail_url}
      alt={props.file.name}
      className='size-12 rounded-md object-cover'
      loading='lazy'
      decoding='async'
    />
  )
}

export default function UploadManagement() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const [category, setCategory] = useState<UploadCategory>('')
  const [page, setPage] = useState(1)
  const [selectedPaths, setSelectedPaths] = useState<Set<string>>(new Set())

  const filesQuery = useQuery({
    queryKey: ['upload-management', 'files', category, page],
    queryFn: () => getUploadedFiles({ category: category || undefined, page }),
    placeholderData: keepPreviousData,
  })
  const statsQuery = useQuery({
    queryKey: ['upload-management', 'stats'],
    queryFn: getUploadStats,
  })

  const files = filesQuery.data?.data ?? []
  const total = filesQuery.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const allPageSelected =
    files.length > 0 && files.every((file) => selectedPaths.has(file.path))

  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({
        queryKey: ['upload-management', 'files'],
      }),
      queryClient.invalidateQueries({
        queryKey: ['upload-management', 'stats'],
      }),
    ])
  }

  const deleteMutation = useMutation({
    mutationFn: deleteFile,
    onSuccess: async () => {
      toast.success(t('File deleted'))
      await refresh()
    },
    onError: () => toast.error(t('Failed to delete file')),
  })
  const batchDeleteMutation = useMutation({
    mutationFn: batchDeleteFiles,
    onSuccess: async (result) => {
      toast.success(t('{{deleted}} files deleted, {{failed}} failed', result))
      setSelectedPaths(new Set())
      await refresh()
    },
    onError: () => toast.error(t('Failed to delete selected files')),
  })
  const clearMutation = useMutation({
    mutationFn: clearUploadedFiles,
    onSuccess: async (result) => {
      toast.success(t('{{count}} files deleted', { count: result.deleted }))
      setSelectedPaths(new Set())
      setPage(1)
      await refresh()
    },
    onError: () => toast.error(t('Failed to clear uploaded files')),
  })

  const togglePath = (path: string) => {
    setSelectedPaths((current) => {
      const next = new Set(current)
      if (next.has(path)) next.delete(path)
      else next.add(path)
      return next
    })
  }

  const stats: StorageStats | undefined = statsQuery.data

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        {t('Upload Management')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <AlertDialog>
          <AlertDialogTrigger
            render={
              <Button
                variant='destructive'
                disabled={total === 0 || clearMutation.isPending}
              />
            }
          >
            <Trash2 data-icon='inline-start' />
            {t('Clear all uploaded files')}
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogMedia>
                <Trash2 aria-hidden='true' />
              </AlertDialogMedia>
              <AlertDialogTitle>
                {t('Clear all uploaded files?')}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t(
                  'This permanently deletes all {{count}} uploaded files and cannot be undone.',
                  { count: stats?.total.count ?? total }
                )}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
              <AlertDialogAction
                variant='destructive'
                onClick={() => clearMutation.mutate()}
              >
                {t('Delete all')}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex h-full min-h-0 flex-col gap-4'>
          <div className='grid grid-cols-2 gap-3 lg:grid-cols-4'>
            <StatCard title={t('Total files')} value={stats?.total} />
            <StatCard title={t('Regular uploads')} value={stats?.uploads} />
            <StatCard title={t('Element assets')} value={stats?.elements} />
            <StatCard title={t('Temporary files')} value={stats?.temp} />
          </div>

          <Card className='min-h-0 flex-1 overflow-hidden'>
            <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-3 border-b'>
              <Select
                value={category || 'all'}
                onValueChange={(value) => {
                  setCategory(value === 'all' ? '' : (value as UploadCategory))
                  setPage(1)
                  setSelectedPaths(new Set())
                }}
              >
                <SelectTrigger className='w-full sm:w-56'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value='all'>{t('All directories')}</SelectItem>
                    <SelectItem value='uploads'>
                      {t('Regular uploads')}
                    </SelectItem>
                    <SelectItem value='elements'>
                      {t('Element assets')}
                    </SelectItem>
                    <SelectItem value='temp'>{t('Temporary files')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <div className='flex flex-wrap gap-2'>
                <Button
                  variant='destructive'
                  disabled={
                    selectedPaths.size === 0 || batchDeleteMutation.isPending
                  }
                  onClick={() => batchDeleteMutation.mutate([...selectedPaths])}
                >
                  <Trash2 data-icon='inline-start' />
                  {t('Delete selected')} ({selectedPaths.size})
                </Button>
                <Button variant='outline' onClick={() => void refresh()}>
                  <RefreshCw data-icon='inline-start' />
                  {t('Refresh')}
                </Button>
              </div>
            </CardHeader>

            <CardContent className='flex min-h-0 flex-1 flex-col p-0'>
              <div className='h-[min(58vh,640px)] overflow-auto'>
                {filesQuery.isPending && (
                  <div className='grid gap-3 p-4'>
                    {Array.from({ length: 7 }, (_, index) => (
                      <Skeleton key={index} className='h-16 w-full' />
                    ))}
                  </div>
                )}
                {filesQuery.isError && (
                  <Empty className='h-full'>
                    <EmptyHeader>
                      <EmptyMedia variant='icon'>
                        <FolderOpen />
                      </EmptyMedia>
                      <EmptyTitle>
                        {t('Failed to load uploaded files')}
                      </EmptyTitle>
                      <EmptyDescription>
                        {t('Refresh the page and try again.')}
                      </EmptyDescription>
                    </EmptyHeader>
                  </Empty>
                )}
                {!filesQuery.isPending &&
                  !filesQuery.isError &&
                  files.length === 0 && (
                    <Empty className='h-full'>
                      <EmptyHeader>
                        <EmptyMedia variant='icon'>
                          <FolderOpen />
                        </EmptyMedia>
                        <EmptyTitle>{t('No uploaded files')}</EmptyTitle>
                        <EmptyDescription>
                          {t('Files will appear here after they are uploaded.')}
                        </EmptyDescription>
                      </EmptyHeader>
                    </Empty>
                  )}
                {!filesQuery.isPending &&
                  !filesQuery.isError &&
                  files.length > 0 && (
                    <>
                      <div className='hidden min-w-[760px] md:block'>
                        <Table>
                          <TableHeader className='bg-background sticky top-0 z-10'>
                            <TableRow>
                              <TableHead className='w-12'>
                                <Checkbox
                                  checked={allPageSelected}
                                  aria-label={t('Select current page')}
                                  onCheckedChange={() => {
                                    setSelectedPaths((current) => {
                                      const next = new Set(current)
                                      for (const file of files) {
                                        if (allPageSelected) {
                                          next.delete(file.path)
                                        } else {
                                          next.add(file.path)
                                        }
                                      }
                                      return next
                                    })
                                  }}
                                />
                              </TableHead>
                              <TableHead>{t('Preview')}</TableHead>
                              <TableHead>{t('File name')}</TableHead>
                              <TableHead>{t('Directory')}</TableHead>
                              <TableHead>{t('Size')}</TableHead>
                              <TableHead>{t('Modified time')}</TableHead>
                              <TableHead className='text-right'>
                                {t('Actions')}
                              </TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {files.map((file) => (
                              <TableRow key={file.path}>
                                <TableCell>
                                  <Checkbox
                                    checked={selectedPaths.has(file.path)}
                                    onCheckedChange={() =>
                                      togglePath(file.path)
                                    }
                                  />
                                </TableCell>
                                <TableCell>
                                  <FilePreview
                                    file={file}
                                    fileLabel={t('File')}
                                  />
                                </TableCell>
                                <TableCell className='max-w-64'>
                                  <a
                                    href={file.url}
                                    target='_blank'
                                    rel='noreferrer'
                                    className='block truncate font-medium hover:underline'
                                  >
                                    {file.name}
                                  </a>
                                </TableCell>
                                <TableCell>
                                  <Badge variant='secondary'>
                                    {file.category}
                                  </Badge>
                                </TableCell>
                                <TableCell className='tabular-nums'>
                                  {formatFileSize(file.size)}
                                </TableCell>
                                <TableCell className='text-muted-foreground whitespace-nowrap'>
                                  {new Intl.DateTimeFormat(
                                    toIntlLocale(i18n.resolvedLanguage),
                                    { dateStyle: 'medium', timeStyle: 'short' }
                                  ).format(file.mod_time * 1000)}
                                </TableCell>
                                <TableCell className='text-right'>
                                  <Button
                                    variant='ghost'
                                    size='sm'
                                    onClick={() =>
                                      deleteMutation.mutate(file.path)
                                    }
                                  >
                                    {t('Delete')}
                                  </Button>
                                </TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      </div>

                      <div className='grid gap-2 p-3 md:hidden'>
                        {files.map((file) => (
                          <div
                            key={file.path}
                            className='flex min-w-0 items-center gap-3 rounded-lg border p-3'
                          >
                            <Checkbox
                              checked={selectedPaths.has(file.path)}
                              onCheckedChange={() => togglePath(file.path)}
                            />
                            <FilePreview file={file} fileLabel={t('File')} />
                            <div className='min-w-0 flex-1'>
                              <a
                                href={file.url}
                                target='_blank'
                                rel='noreferrer'
                                className='block truncate text-sm font-medium'
                              >
                                {file.name}
                              </a>
                              <div className='text-muted-foreground mt-1 flex flex-wrap items-center gap-2 text-xs'>
                                <Badge variant='secondary'>
                                  {file.category}
                                </Badge>
                                <span>{formatFileSize(file.size)}</span>
                              </div>
                            </div>
                            <Button
                              variant='ghost'
                              size='icon-sm'
                              aria-label={t('Delete')}
                              onClick={() => deleteMutation.mutate(file.path)}
                            >
                              <Trash2 />
                            </Button>
                          </div>
                        ))}
                      </div>
                    </>
                  )}
              </div>

              <div className='flex flex-col gap-2 border-t p-3 sm:flex-row sm:items-center sm:justify-between'>
                <span className='text-muted-foreground text-sm'>
                  {t('{{total}} files, page {{page}} of {{pages}}', {
                    total,
                    page,
                    pages: totalPages,
                  })}
                </span>
                <div className='flex gap-2'>
                  <Button
                    variant='outline'
                    size='sm'
                    disabled={page <= 1 || filesQuery.isFetching}
                    onClick={() =>
                      setPage((current) => Math.max(1, current - 1))
                    }
                  >
                    {t('Previous')}
                  </Button>
                  <Button
                    variant='outline'
                    size='sm'
                    disabled={page >= totalPages || filesQuery.isFetching}
                    onClick={() => setPage((current) => current + 1)}
                  >
                    {t('Next')}
                  </Button>
                </div>
              </div>
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
