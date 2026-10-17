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

import { useMutation, useQuery } from '@tanstack/react-query'
import { ImagePlus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { handleServerError } from '@/lib/handle-server-error'

import {
  getImageGenerationGroups,
  getImageGenerationModels,
  getImageGenerationTask,
  submitImageGeneration,
  uploadReferenceImage,
} from './api'
import { GenerationForm } from './components/generation-form'
import { GenerationResult } from './components/generation-result'
import {
  buildImageGenerationRequest,
  isTerminalTaskStatus,
} from './lib/image-generation'
import type { ImageGenerationFormValue, ReferenceImage } from './types'

export function ImageGeneration() {
  const { t } = useTranslation()
  const [group, setGroup] = useState('')
  const [taskId, setTaskId] = useState('')

  const groupsQuery = useQuery({
    queryKey: ['image-generation', 'groups'],
    queryFn: getImageGenerationGroups,
  })
  const modelsQuery = useQuery({
    queryKey: ['image-generation', 'models', group],
    queryFn: () => getImageGenerationModels(group),
    enabled: group !== '',
  })
  const taskQuery = useQuery({
    queryKey: ['image-generation', 'task', taskId],
    queryFn: () => getImageGenerationTask(taskId),
    enabled: taskId !== '',
    refetchInterval: (query) =>
      isTerminalTaskStatus(query.state.data?.status) ? false : 1500,
  })

  const generationMutation = useMutation({
    mutationFn: async (input: {
      references: ReferenceImage[]
      value: ImageGenerationFormValue
    }) => {
      const uploaded = await Promise.all(
        input.references.map((reference) =>
          uploadReferenceImage(reference.file)
        )
      )
      const request = buildImageGenerationRequest(
        input.value,
        uploaded.map((attachment) => attachment.url)
      )
      return submitImageGeneration(request)
    },
    onMutate: () => setTaskId(''),
    onSuccess: (submission) => setTaskId(submission.task_id),
    onError: (error) => handleServerError(error, t('Image generation failed')),
  })

  const taskIsActive =
    taskId !== '' && !isTerminalTaskStatus(taskQuery.data?.status)
  const disabled = generationMutation.isPending || taskIsActive

  return (
    <div className='bg-muted/20 flex size-full min-h-0 flex-col overflow-auto'>
      <div className='mx-auto flex w-full max-w-7xl flex-1 flex-col gap-5 p-4 sm:p-6'>
        <header className='flex items-start gap-3'>
          <div className='bg-primary/10 text-primary flex size-10 shrink-0 items-center justify-center rounded-xl'>
            <ImagePlus className='size-5' aria-hidden='true' />
          </div>
          <div>
            <h1 className='text-2xl font-semibold tracking-tight'>
              {t('Image Generation')}
            </h1>
            <p className='text-muted-foreground mt-1 text-sm'>
              {t('Create and edit images with Nano Banana and GPT Image.')}
            </p>
          </div>
        </header>

        <div className='grid min-h-0 flex-1 gap-5 lg:grid-cols-[360px_minmax(0,1fr)]'>
          <Card className='h-fit'>
            <CardHeader className='border-b'>
              <CardTitle>{t('Generation settings')}</CardTitle>
            </CardHeader>
            <CardContent>
              <GenerationForm
                disabled={disabled}
                groups={groupsQuery.data ?? []}
                isLoadingModels={modelsQuery.isLoading}
                models={modelsQuery.data ?? []}
                onGroupChange={setGroup}
                onSubmit={(value, references) =>
                  generationMutation.mutate({ references, value })
                }
              />
            </CardContent>
          </Card>

          <Card className='min-h-[520px] gap-0 py-0'>
            <GenerationResult
              isError={taskQuery.isError}
              isLoading={generationMutation.isPending}
              onRetry={() => void taskQuery.refetch()}
              task={taskQuery.data}
            />
          </Card>
        </div>
      </div>
    </div>
  )
}
