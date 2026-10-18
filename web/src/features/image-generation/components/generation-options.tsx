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

import { useFormContext, type Control } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { isGeminiFlashImageModel } from '../lib/image-generation'
import type { ImageGenerationFormValue, ImageModelFamily } from '../types'

type GenerationOptionsProps = {
  control: Control<ImageGenerationFormValue>
  disabled: boolean
  family: ImageModelFamily | null
  model: string
}

const BANANA_SIZES = ['1K', '2K', '4K']
const GPT_SIZES = [
  'auto',
  '1024x1024',
  '1536x1024',
  '1360x1024',
  '1824x1024',
  '1024x1536',
  '1024x1360',
  '1024x1824',
  '2048x2048',
  '3072x2048',
  '2736x2048',
  '3648x2048',
  '2048x3072',
  '2048x2736',
  '2048x3648',
  '2880x2880',
  '3504x2336',
  '3264x2448',
  '3840x2160',
  '2336x3504',
  '2448x3264',
  '2160x3840',
]
const BASE_ASPECT_RATIOS = [
  '1:1',
  '2:3',
  '3:2',
  '3:4',
  '4:3',
  '4:5',
  '5:4',
  '9:16',
  '16:9',
  '21:9',
]
const FLASH_ASPECT_RATIOS = [...BASE_ASPECT_RATIOS, '1:4', '1:8', '4:1', '8:1']
const QUALITIES = ['auto', 'low', 'medium', 'high', 'xhigh', 'max']
const OUTPUT_FORMATS = ['png', 'jpeg', 'webp'] as const

function OptionSelect(props: {
  disabled: boolean
  label: string
  onChange: (value: string) => void
  options: readonly string[]
  value: string
}) {
  return (
    <Select
      modal={false}
      items={props.options.map((value) => ({ label: value, value }))}
      value={props.value}
      onValueChange={(value) => value && props.onChange(value)}
      disabled={props.disabled}
    >
      <SelectTrigger
        className='w-full transition-none'
        aria-label={props.label}
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent alignItemWithTrigger={false} animated={false}>
        <SelectGroup>
          {props.options.map((value) => (
            <SelectItem key={value} value={value}>
              {value}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}

export function GenerationOptions(props: GenerationOptionsProps) {
  const { t } = useTranslation()
  const form = useFormContext<ImageGenerationFormValue>()
  const sizeOptions = props.family === 'banana' ? BANANA_SIZES : GPT_SIZES
  const normalizedModel = props.model.toLowerCase()
  const supportsExtendedQuality =
    normalizedModel.startsWith('gpt-image-2.5-sunburst') ||
    normalizedModel.startsWith('gpt-image-2.5-flare')
  const qualityOptions = supportsExtendedQuality
    ? QUALITIES
    : QUALITIES.slice(0, 4)
  const flashImageModel = isGeminiFlashImageModel(props.model)
  const aspectRatioOptions = flashImageModel
    ? FLASH_ASPECT_RATIOS
    : BASE_ASPECT_RATIOS

  return (
    <div className='grid grid-cols-1 gap-3 @sm:grid-cols-2'>
      <FormField
        control={props.control}
        name='size'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Output image size')}</FormLabel>
            <FormControl>
              <OptionSelect
                disabled={props.disabled}
                label={t('Output image size')}
                onChange={field.onChange}
                options={sizeOptions}
                value={field.value}
              />
            </FormControl>
          </FormItem>
        )}
      />

      {props.family === 'banana' ? (
        <FormField
          control={props.control}
          name='aspectRatio'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Output aspect ratio')}</FormLabel>
              <FormControl>
                <OptionSelect
                  disabled={props.disabled}
                  label={t('Output aspect ratio')}
                  onChange={field.onChange}
                  options={aspectRatioOptions}
                  value={field.value}
                />
              </FormControl>
            </FormItem>
          )}
        />
      ) : (
        <FormField
          control={props.control}
          name='quality'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Quality')}</FormLabel>
              <FormControl>
                <OptionSelect
                  disabled={props.disabled}
                  label={t('Quality')}
                  onChange={field.onChange}
                  options={qualityOptions}
                  value={field.value}
                />
              </FormControl>
            </FormItem>
          )}
        />
      )}

      {props.family === 'gpt-image' ? (
        <>
          <FormField
            control={props.control}
            name='outputFormat'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Output format')}</FormLabel>
                <FormControl>
                  <OptionSelect
                    disabled={props.disabled}
                    label={t('Output format')}
                    onChange={(value) => {
                      field.onChange(value)
                      if (
                        value === 'jpeg' &&
                        form.getValues('background') === 'transparent'
                      ) {
                        form.setValue('background', 'auto')
                      }
                    }}
                    options={OUTPUT_FORMATS}
                    value={field.value}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <FormField
            control={props.control}
            name='background'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Image background')}</FormLabel>
                <FormControl>
                  <OptionSelect
                    disabled={props.disabled}
                    label={t('Image background')}
                    onChange={(value) => {
                      field.onChange(value)
                      if (
                        value === 'transparent' &&
                        form.getValues('outputFormat') === 'jpeg'
                      ) {
                        form.setValue('outputFormat', 'png')
                      }
                    }}
                    options={['auto', 'transparent']}
                    value={field.value}
                  />
                </FormControl>
              </FormItem>
            )}
          />

        </>
      ) : null}

      {props.family === 'banana' && flashImageModel ? (
        <FormField
          control={props.control}
          name='thinkingLevel'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Thinking level')}</FormLabel>
              <FormControl>
                <OptionSelect
                  disabled={props.disabled}
                  label={t('Thinking level')}
                  onChange={field.onChange}
                  options={['default', 'minimal', 'high']}
                  value={field.value}
                />
              </FormControl>
            </FormItem>
          )}
        />
      ) : null}
    </div>
  )
}
