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
import { Film, Music, Gamepad2, Video } from 'lucide-react'
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

interface KlingModelCardProps {
  title: string
  description: string
  options: PricingOption[]
  className?: string
}

function KlingModelCard({ title, description, options, className }: KlingModelCardProps) {
  return (
    <div
      className={cn(
        'bg-card border-border group relative overflow-hidden rounded-lg border p-5 shadow-sm transition-all hover:shadow-md',
        className
      )}
    >
      <div className='mb-4'>
        <h3 className='text-foreground mb-1 text-lg font-semibold'>{title}</h3>
        <p className='text-muted-foreground text-sm'>{description}</p>
      </div>

      <div className='space-y-4'>
        {options.map((option, index) => (
          <div key={index} className='space-y-1.5'>
            <div className='text-foreground flex items-center gap-2 text-sm font-medium'>
              {option.icon}
              <span>{option.label}</span>
            </div>
            <div className='text-muted-foreground ml-6 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs'>
              {option.prices['720P'] && (
                <span className='bg-muted/50 rounded px-2 py-0.5'>
                  720P: <span className='text-foreground font-medium'>{option.prices['720P']}</span>
                </span>
              )}
              {option.prices['1080P'] && (
                <span className='bg-muted/50 rounded px-2 py-0.5'>
                  1080P: <span className='text-foreground font-medium'>{option.prices['1080P']}</span>
                </span>
              )}
              {option.prices['4K'] && (
                <span className='bg-muted/50 rounded px-2 py-0.5'>
                  4K: <span className='text-foreground font-medium'>{option.prices['4K']}</span>
                </span>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

export function KlingPricingCards() {
  const { t } = useTranslation()

  const klingV3Options: PricingOption[] = [
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

  const klingV3OmniOptions: PricingOption[] = [
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

  const klingV26Options: PricingOption[] = [
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

  return (
    <div className='space-y-4'>
      <div>
        <h2 className='text-foreground mb-1 text-xl font-semibold'>
          🎬 {t('Kling Video Models')}
        </h2>
        <p className='text-muted-foreground text-sm'>
          {t('Billing: Per second, based on actual video duration')}
        </p>
      </div>

      <div className='grid grid-cols-1 gap-4 lg:grid-cols-3'>
        <KlingModelCard
          title='Kling v3'
          description={t('Latest generation • High quality • Multiple resolutions')}
          options={klingV3Options}
        />

        <KlingModelCard
          title='Kling v3 Omni'
          description={t('Multimodal • Video input support • Flexible options')}
          options={klingV3OmniOptions}
        />

        <KlingModelCard
          title='Kling v2-6'
          description={t('Cost-effective • Proven quality')}
          options={klingV26Options}
        />
      </div>
    </div>
  )
}
