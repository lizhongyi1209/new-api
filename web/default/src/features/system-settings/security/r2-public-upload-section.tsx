/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Copy, Plus, RotateCw, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import {
  getR2PublicUploadClients,
  rotateR2PublicUploadSecret,
  updateR2PublicUploadClients,
} from '../api'
import { SettingsSection } from '../components/settings-section'
import type { R2PublicUploadClient } from '../types'

const queryKey = ['system-settings', 'r2-public-upload-clients'] as const

export function R2PublicUploadSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { copyToClipboard } = useCopyToClipboard({
    successMessage: t('New signing secret copied'),
  })
  const [clients, setClients] = useState<R2PublicUploadClient[]>([])
  const [rotateClientId, setRotateClientId] = useState<string | null>(null)
  const query = useQuery({
    queryKey,
    queryFn: getR2PublicUploadClients,
  })

  useEffect(() => {
    if (query.data?.data) setClients(query.data.data)
  }, [query.data])

  const saveMutation = useMutation({
    mutationFn: updateR2PublicUploadClients,
    onSuccess: async () => {
      toast.success(t('R2 upload whitelist updated'))
      await queryClient.invalidateQueries({ queryKey })
    },
  })
  const rotateMutation = useMutation({
    mutationFn: rotateR2PublicUploadSecret,
    onSuccess: async (response) => {
      setRotateClientId(null)
      await copyToClipboard(response.data.secret)
      await queryClient.invalidateQueries({ queryKey })
    },
  })

  const updateClient = (
    index: number,
    patch: Partial<R2PublicUploadClient>
  ) => {
    setClients((current) =>
      current.map((client, itemIndex) =>
        itemIndex === index ? { ...client, ...patch } : client
      )
    )
  }

  return (
    <SettingsSection title={t('R2 Upload Whitelist')}>
      <div className='space-y-5'>
        <div className='bg-muted/30 text-muted-foreground rounded-lg border p-4 text-sm'>
          <p>
            {t(
              'Allow trusted downstream servers to request short-lived Cloudflare R2 upload URLs.'
            )}
          </p>
          <p className='text-foreground mt-2 font-mono text-xs'>
            POST /v1/storage/public/presign
          </p>
          <p className='mt-2'>
            {t(
              'Each request must include the allowed Origin and a valid HMAC signature.'
            )}
          </p>
        </div>

        {clients.map((client, index) => (
          <div key={client.id} className='space-y-4 rounded-xl border p-4'>
            <div className='flex items-center justify-between gap-3'>
              <Input
                aria-label={t('Downstream name')}
                value={client.name}
                onChange={(event) =>
                  updateClient(index, { name: event.target.value })
                }
                className='max-w-sm font-medium'
              />
              <div className='flex items-center gap-2'>
                <Label htmlFor={`r2-client-${index}`}>{t('Enabled')}</Label>
                <Switch
                  id={`r2-client-${index}`}
                  checked={client.enabled}
                  onCheckedChange={(enabled) =>
                    updateClient(index, { enabled })
                  }
                />
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  aria-label={t('Remove downstream')}
                  onClick={() =>
                    setClients((current) =>
                      current.filter((_, itemIndex) => itemIndex !== index)
                    )
                  }
                >
                  <Trash2 className='size-4' />
                </Button>
              </div>
            </div>

            <div className='grid gap-4 md:grid-cols-2'>
              <div className='space-y-2'>
                <Label>{t('Downstream ID')}</Label>
                <Input
                  value={client.id}
                  onChange={(event) =>
                    updateClient(index, {
                      id: event.target.value.toLowerCase(),
                    })
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label>{t('Allowed origins')}</Label>
                <Textarea
                  value={client.origins.join('\n')}
                  onChange={(event) =>
                    updateClient(index, {
                      origins: event.target.value.split('\n'),
                    })
                  }
                  placeholder='https://api.example.com'
                />
                <p className='text-muted-foreground text-xs'>
                  {t('One HTTPS origin per line.')}
                </p>
              </div>
              <div className='space-y-2'>
                <Label>{t('Max file size (MB)')}</Label>
                <Input
                  type='number'
                  min={1}
                  max={100}
                  value={client.max_file_size_mb}
                  onChange={(event) =>
                    updateClient(index, {
                      max_file_size_mb: Number(event.target.value),
                    })
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label>{t('Requests per minute')}</Label>
                <Input
                  type='number'
                  min={1}
                  max={600}
                  value={client.requests_per_minute}
                  onChange={(event) =>
                    updateClient(index, {
                      requests_per_minute: Number(event.target.value),
                    })
                  }
                />
              </div>
            </div>

            <div className='flex flex-wrap items-center gap-2 border-t pt-4'>
              <Button
                type='button'
                variant='outline'
                disabled={!client.has_secret || rotateMutation.isPending}
                onClick={() => setRotateClientId(client.id)}
              >
                {client.has_secret ? (
                  <RotateCw className='size-4' />
                ) : (
                  <Copy className='size-4' />
                )}
                {t('Rotate and copy signing secret')}
              </Button>
              <span className='text-muted-foreground text-xs'>
                {t(
                  'The secret is returned only once. Rotation immediately invalidates the old secret.'
                )}
              </span>
            </div>
          </div>
        ))}

        <div className='flex flex-wrap justify-between gap-3'>
          <Button
            type='button'
            variant='outline'
            onClick={() =>
              setClients((current) => [
                ...current,
                {
                  id: `downstream-${current.length + 1}`,
                  name: t('New downstream'),
                  origins: [''],
                  enabled: true,
                  max_file_size_mb: 100,
                  requests_per_minute: 20,
                  has_secret: false,
                },
              ])
            }
          >
            <Plus className='size-4' />
            {t('Add downstream')}
          </Button>
          <Button
            type='button'
            disabled={query.isLoading || saveMutation.isPending}
            onClick={() => saveMutation.mutate(clients)}
          >
            {t('Save whitelist')}
          </Button>
        </div>
        <ConfirmDialog
          open={rotateClientId !== null}
          onOpenChange={(open) => !open && setRotateClientId(null)}
          title={t('Rotate and copy signing secret')}
          desc={t(
            'The secret is returned only once. Rotation immediately invalidates the old secret.'
          )}
          confirmText={t('Rotate and copy signing secret')}
          isLoading={rotateMutation.isPending}
          handleConfirm={() =>
            rotateClientId && rotateMutation.mutate(rotateClientId)
          }
        />
      </div>
    </SettingsSection>
  )
}
