import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StaticDataTable } from '@/components/data-table'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useAuthStore } from '@/stores/auth-store'

import { resellerGet, setResellerAccess, type Customer } from './api'

const schema = z.object({ username: z.string().trim().min(1).max(64) })
export function ResellerAccess() {
  const { t } = useTranslation()
  const owner = useAuthStore((state) => state.auth.user?.id)
  const client = useQueryClient()
  const [revoke, setRevoke] = useState<Customer | null>(null)
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: { username: '' },
  })
  const query = useQuery({
    queryKey: ['reseller', 'access', owner],
    queryFn: ({ signal }) =>
      resellerGet<Customer[]>('access', undefined, signal),
    meta: { errorToast: false },
  })
  const mutation = useMutation({
    mutationFn: ({
      username,
      enabled,
    }: {
      username: string
      enabled: boolean
    }) => setResellerAccess(username, enabled),
    onSuccess: async () => {
      form.reset()
      setRevoke(null)
      await client.invalidateQueries({ queryKey: ['reseller', 'access'] })
    },
  })
  if (query.isPending) return <LoadingState />
  if (query.isError) return <ErrorState onRetry={() => void query.refetch()} />
  return (
    <section aria-label={t('Reseller access')}>
      <form
        className='ledger-toolbar'
        onSubmit={form.handleSubmit((value) =>
          mutation.mutate({ username: value.username, enabled: true })
        )}
      >
        <Input
          aria-label={t('Reseller username')}
          placeholder={t('Reseller username')}
          {...form.register('username')}
          aria-invalid={Boolean(form.formState.errors.username)}
        />
        <Button type='submit' disabled={mutation.isPending}>
          {t('Grant access')}
        </Button>
      </form>
      <StaticDataTable
        className='ledger-table'
        data={query.data}
        getRowKey={(row) => row.id}
        emptyContent={t('No reseller accounts')}
        columns={[
          {
            id: 'username',
            header: t('Customer name'),
            cell: (row) => row.username,
          },
          { id: 'id', header: 'ID', cell: (row) => row.id },
          {
            id: 'action',
            header: '',
            cell: (row) => (
              <Button
                variant='link'
                disabled={mutation.isPending}
                onClick={() => setRevoke(row)}
              >
                {t('Revoke access')}
              </Button>
            ),
          },
        ]}
      />
      <ConfirmDialog
        open={Boolean(revoke)}
        onOpenChange={(open) => {
          if (!open) setRevoke(null)
        }}
        title={t('Revoke reseller access')}
        desc={t('Revoke access for {{name}}?', { name: revoke?.username })}
        confirmText={t('Revoke access')}
        isLoading={mutation.isPending}
        handleConfirm={() => {
          if (revoke) {
            mutation.mutate({ username: revoke.username, enabled: false })
          }
        }}
      />
    </section>
  )
}
