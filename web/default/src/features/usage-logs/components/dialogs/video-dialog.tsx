import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Skeleton } from '@/components/ui/skeleton'

interface VideoDialogProps {
  videoUrl: string
  taskId?: string
  prompt?: string
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function VideoDialog({
  videoUrl,
  taskId,
  prompt,
  open,
  onOpenChange,
}: VideoDialogProps) {
  const { t } = useTranslation()
  const [isLoading, setIsLoading] = useState(true)
  const [hasError, setHasError] = useState(false)

  const handleOpenChange = (newOpen: boolean) => {
    if (newOpen) {
      setIsLoading(true)
      setHasError(false)
    }
    onOpenChange(newOpen)
  }

  const handleLoadedData = () => {
    setIsLoading(false)
    setHasError(false)
  }

  const handleVideoError = () => {
    setIsLoading(false)
    setHasError(true)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className='sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Video Preview')}</DialogTitle>
          <DialogDescription>
            {taskId
              ? `${t('Task ID:')} ${taskId}`
              : t('View the generated video')}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='max-h-[600px]'>
          <div className='py-4'>
            <div className='bg-muted/50 relative flex min-h-[300px] items-center justify-center rounded-lg border'>
              {(isLoading || hasError) && (
                <Skeleton className='absolute inset-0 h-full w-full rounded-lg' />
              )}

              <video
                src={videoUrl}
                controls
                className={`max-h-[550px] w-full rounded-lg ${
                  isLoading || hasError ? 'opacity-0' : 'opacity-100'
                }`}
                onLoadedData={handleLoadedData}
                onError={handleVideoError}
                preload='metadata'
              >
                {t('Your browser does not support video playback.')}
              </video>

              {hasError && (
                <div className='absolute inset-0 flex items-center justify-center'>
                  <p className='text-muted-foreground text-sm'>
                    {t('Failed to load video')}
                  </p>
                </div>
              )}
            </div>

            {/* Prompt */}
            {prompt && (
              <div className='mt-4 space-y-1'>
                <p className='text-muted-foreground text-xs font-medium'>{t('Prompt')}</p>
                <div className='bg-muted rounded-md p-3'>
                  <p className='text-foreground text-sm whitespace-pre-wrap break-all'>
                    {prompt}
                  </p>
                </div>
              </div>
            )}

            {/* Video URL */}
            <div className='bg-muted mt-4 rounded-md p-3'>
              <p className='text-muted-foreground font-mono text-xs break-all'>
                {videoUrl}
              </p>
            </div>
          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
