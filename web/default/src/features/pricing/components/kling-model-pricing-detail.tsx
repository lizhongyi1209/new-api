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
import { Film, Music, Gamepad2, Video, Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

interface PricingOption {
  icon: React.ReactNode
  label: string
  prices: {
    '720P'?: string
    '1080P'?: string
    '4K'?: string
  }
}

interface KlingModelPricingDetailProps {
  modelName: string
  className?: string
}

function getPricingOptions(modelName: string, t: (key: string) => string): PricingOption[] | null {
  // Kling v3 models
  if (modelName.includes('kling-v3') && !modelName.includes('omni')) {
    return [
      {
        icon: <Film className='size-4' />,
        label: t('Text/Image to Video'),
        prices: {
          '720P': '$0.61/s',
          '1080P': '$0.82/s',
          '4K': '$3.07/s',
        },
      },
      {
        icon: <Music className='size-4' />,
        label: t('With Audio Generation'),
        prices: {
          '720P': '$0.92/s',
          '1080P': '$1.23/s',
          '4K': '$3.07/s',
        },
      },
      {
        icon: <Gamepad2 className='size-4' />,
        label: t('Motion Control'),
        prices: {
          '720P': '$0.92/s',
          '1080P': '$1.23/s',
        },
      },
    ]
  }

  // Kling v3 Omni models
  if (modelName.includes('kling-v3') && modelName.includes('omni')) {
    return [
      {
        icon: <Video className='size-4' />,
        label: t('With Video Input'),
        prices: {
          '720P': '$0.61/s',
          '1080P': '$0.82/s',
          '4K': '$3.07/s',
        },
      },
      {
        icon: <Video className='size-4' />,
        label: t('With Video Input + Audio'),
        prices: {
          '720P': '$0.92/s',
          '1080P': '$1.23/s',
          '4K': '$3.07/s',
        },
      },
      {
        icon: <Music className='size-4' />,
        label: t('No Video Input + Audio'),
        prices: {
          '720P': '$0.82/s',
          '1080P': '$1.02/s',
          '4K': '$3.07/s',
        },
      },
    ]
  }

  // Kling v2-6 models
  if (modelName.includes('kling-v2-6')) {
    return [
      {
        icon: <Film className='size-4' />,
        label: t('Standard'),
        prices: {
          '720P': '$0.31/s',
          '1080P': '$0.51/s',
        },
      },
      {
        icon: <Music className='size-4' />,
        label: t('+ Audio (No Voice Control)'),
        prices: {
          '1080P': '$1.02/s',
        },
      },
      {
        icon: <Music className='size-4' />,
        label: t('+ Audio (Voice Control)'),
        prices: {
          '1080P': '$1.23/s',
        },
      },
      {
        icon: <Gamepad2 className='size-4' />,
        label: t('Motion Control'),
        prices: {
          '720P': '$0.51/s',
          '1080P': '$0.82/s',
        },
      },
    ]
  }

  return null
}

export function KlingModelPricingDetail({ modelName, className }: KlingModelPricingDetailProps) {
  const { t } = useTranslation()
  const pricingOptions = getPricingOptions(modelName, t)

  if (!pricingOptions) {
    return null
  }

  return (
    <div className={cn('bg-blue-50/50 dark:bg-blue-950/20 space-y-4 rounded-lg border border-blue-200/50 dark:border-blue-800/30 p-4', className)}>
      <div className='flex items-start gap-2'>
        <Info className='text-blue-600 dark:text-blue-400 mt-0.5 size-4 flex-shrink-0' />
        <div className='flex-1 space-y-1'>
          <h4 className='text-foreground text-sm font-semibold'>
            {t('Detailed Pricing by Resolution & Features')}
          </h4>
          <p className='text-muted-foreground text-xs'>
            {t('Actual charges based on final video duration and selected options')}
          </p>
        </div>
      </div>

      <div className='space-y-3'>
        {pricingOptions.map((option, index) => (
          <div key={index} className='space-y-1.5'>
            <div className='text-foreground flex items-center gap-2 text-sm font-medium'>
              {option.icon}
              <span>{option.label}</span>
            </div>
            <div className='text-muted-foreground ml-6 flex flex-wrap items-center gap-2 text-xs'>
              {option.prices['720P'] && (
                <span className='bg-muted/70 dark:bg-muted/40 rounded-md px-2.5 py-1'>
                  <span className='text-muted-foreground/70 mr-1'>720P:</span>
                  <span className='text-foreground font-semibold'>{option.prices['720P']}</span>
                </span>
              )}
              {option.prices['1080P'] && (
                <span className='bg-muted/70 dark:bg-muted/40 rounded-md px-2.5 py-1'>
                  <span className='text-muted-foreground/70 mr-1'>1080P:</span>
                  <span className='text-foreground font-semibold'>{option.prices['1080P']}</span>
                </span>
              )}
              {option.prices['4K'] && (
                <span className='bg-muted/70 dark:bg-muted/40 rounded-md px-2.5 py-1'>
                  <span className='text-muted-foreground/70 mr-1'>4K:</span>
                  <span className='text-foreground font-semibold'>{option.prices['4K']}</span>
                </span>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
