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
import { MessageSquareIcon, PlusIcon, SparklesIcon, XIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  PromptInput,
  PromptInputFooter,
  PromptInputTextarea,
  type PromptInputMessage,
} from '@/components/ai-elements/prompt-input'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

import { getSubmittableInputText } from '../../lib'
import type {
  ModelOption,
  GroupOption,
  ImageResponseFormat,
  ParameterEnabled,
  PlaygroundConfig,
  PlaygroundSubmitInput,
} from '../../types'
import { PlaygroundInputControls } from './playground-input-controls'
import { PlaygroundInputTools } from './playground-input-tools'

interface PlaygroundInputProps {
  config: PlaygroundConfig
  onSubmit: (input: PlaygroundSubmitInput) => void
  onStop?: () => void
  disabled?: boolean
  isGenerating?: boolean
  models: ModelOption[]
  modelValue: string
  onModelChange: (value: string) => void
  isModelLoading?: boolean
  groups: GroupOption[]
  groupValue: string
  onGroupChange: (value: string) => void
  hasMessages?: boolean
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
  onClearMessages?: () => void
  onParameterEnabledChange: (
    key: keyof ParameterEnabled,
    value: boolean
  ) => void
  parameterEnabled: ParameterEnabled
}

const imageSizeOptions = [
  '1024x1024',
  '1024x1792',
  '1792x1024',
  '512x512',
  '256x256',
]

const imageQualityOptions = ['auto', 'standard', 'hd', 'low', 'medium', 'high']

const imageResponseFormatOptions: ImageResponseFormat[] = [
  'auto',
  'url',
  'b64_json',
]

const imageResponseFormatLabels: Record<ImageResponseFormat, string> = {
  auto: 'Auto',
  url: 'URL',
  b64_json: 'Base64 JSON',
}

const imageReferenceIds = ['image', 'image2', 'image3'] as const
const imageReferenceLabelKeys = ['Image URL', 'Image2 URL', 'Image3 URL']

const imageCountOptions = [1, 2, 3, 4]

export function PlaygroundInput({
  config,
  onSubmit,
  onStop,
  disabled,
  isGenerating,
  models,
  modelValue,
  onModelChange,
  isModelLoading = false,
  groups,
  groupValue,
  onGroupChange,
  hasMessages = false,
  onConfigChange,
  onClearMessages,
  onParameterEnabledChange,
  parameterEnabled,
}: PlaygroundInputProps) {
  const { t } = useTranslation()
  const [text, setText] = useState('')
  const [imageReferenceUrls, setImageReferenceUrls] = useState(['', ''])
  const isImageMode = config.mode === 'image'

  const handleSubmit = (message: PromptInputMessage) => {
    const submittableText = getSubmittableInputText(message, disabled)

    if (!submittableText) return
    onSubmit({
      text: submittableText,
      imageReferenceUrls: isImageMode ? imageReferenceUrls : undefined,
    })
    setText('')
  }

  const updateImageReferenceUrl = (index: number, value: string) => {
    setImageReferenceUrls((previous) =>
      previous.map((url, currentIndex) =>
        currentIndex === index ? value : url
      )
    )
  }

  return (
    <div className='grid shrink-0 gap-4 px-1 md:pb-4'>
      <PromptInput
        className='relative'
        groupClassName='bg-background/95 dark:bg-background/80 border-border/70 shadow-[0_18px_60px_-32px_rgba(0,0,0,0.65)] ring-1 ring-foreground/5 rounded-xl overflow-hidden transition-all duration-200 focus-within:border-primary/45 focus-within:ring-primary/15 focus-within:shadow-[0_22px_70px_-34px_rgba(0,0,0,0.75)]'
        onSubmit={handleSubmit}
      >
        <PromptInputTextarea
          autoComplete='off'
          autoCorrect='off'
          autoCapitalize='off'
          spellCheck={false}
          className='min-h-20 px-5 pt-4 pb-3 leading-7 md:min-h-24 md:text-base'
          disabled={disabled}
          onChange={(event) => setText(event.target.value)}
          placeholder={
            isImageMode ? t('Describe an image to generate') : t('Ask anything')
          }
          value={text}
        />

        {isImageMode && (
          <div className='border-border/60 grid gap-3 border-t px-3 py-3'>
            <div className='grid gap-2 sm:grid-cols-4'>
              <Select
                items={imageSizeOptions.map((value) => ({
                  value,
                  label: value,
                }))}
                value={config.image_size}
                onValueChange={(value) =>
                  value !== null && onConfigChange('image_size', value)
                }
              >
                <SelectTrigger aria-label={t('Size')} className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {imageSizeOptions.map((value) => (
                      <SelectItem key={value} value={value}>
                        {value}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>

              <Select
                items={imageQualityOptions.map((value) => ({
                  value,
                  label: t(value === 'auto' ? 'Auto' : value),
                }))}
                value={config.image_quality}
                onValueChange={(value) =>
                  value !== null && onConfigChange('image_quality', value)
                }
              >
                <SelectTrigger aria-label={t('Quality')} className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {imageQualityOptions.map((value) => (
                      <SelectItem key={value} value={value}>
                        {t(value === 'auto' ? 'Auto' : value)}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>

              <Select
                items={imageResponseFormatOptions.map((value) => ({
                  value,
                  label: t(imageResponseFormatLabels[value]),
                }))}
                value={config.image_response_format}
                onValueChange={(value) =>
                  value !== null &&
                  onConfigChange(
                    'image_response_format',
                    value as ImageResponseFormat
                  )
                }
              >
                <SelectTrigger aria-label={t('Response')} className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {imageResponseFormatOptions.map((value) => (
                      <SelectItem key={value} value={value}>
                        {t(imageResponseFormatLabels[value])}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>

              <Select
                items={imageCountOptions.map((value) => ({
                  value: String(value),
                  label: String(value),
                }))}
                value={String(config.image_n)}
                onValueChange={(value) =>
                  value !== null && onConfigChange('image_n', Number(value))
                }
              >
                <SelectTrigger aria-label={t('Count')} className='w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {imageCountOptions.map((value) => (
                      <SelectItem key={value} value={String(value)}>
                        {value}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </div>

            <div className='grid gap-2'>
              {imageReferenceUrls
                .map((url, index) => ({
                  id: imageReferenceIds[index],
                  index,
                  url,
                }))
                .map(({ id, index, url }) => (
                  <div className='flex items-center gap-2' key={id}>
                    <Input
                      disabled={disabled}
                      onChange={(event) =>
                        updateImageReferenceUrl(index, event.target.value)
                      }
                      placeholder={t(imageReferenceLabelKeys[index])}
                      value={url}
                    />
                    <Button
                      aria-label={t('Remove reference image')}
                      disabled={disabled || imageReferenceUrls.length <= 1}
                      onClick={() =>
                        setImageReferenceUrls((previous) =>
                          previous.length <= 1
                            ? ['']
                            : previous.filter(
                                (_, currentIndex) => currentIndex !== index
                              )
                        )
                      }
                      size='icon'
                      type='button'
                      variant='ghost'
                    >
                      <XIcon />
                    </Button>
                  </div>
                ))}
              {imageReferenceUrls.length < 3 && (
                <Button
                  className='w-fit'
                  disabled={disabled}
                  onClick={() =>
                    setImageReferenceUrls((previous) => [...previous, ''])
                  }
                  size='sm'
                  type='button'
                  variant='outline'
                >
                  <PlusIcon data-icon='inline-start' />
                  {t('Add reference image')}
                </Button>
              )}
            </div>
          </div>
        )}

        <PromptInputFooter className='border-border/60 bg-muted/20 dark:bg-muted/10 border-t px-3 py-2.5 backdrop-blur'>
          <PlaygroundInputControls
            disabled={disabled}
            groups={groups}
            groupValue={groupValue}
            isGenerating={isGenerating}
            isModelLoading={isModelLoading}
            models={models}
            modelValue={modelValue}
            onGroupChange={onGroupChange}
            onModelChange={onModelChange}
            onStop={onStop}
            text={text}
            submitLabel={isImageMode ? t('Generate') : t('Send')}
            tools={
              <div className='flex items-center gap-1'>
                <ToggleGroup
                  disabled={disabled}
                  onValueChange={(value) => {
                    const nextMode = value[0]
                    if (nextMode === 'chat' || nextMode === 'image') {
                      onConfigChange('mode', nextMode)
                    }
                  }}
                  size='sm'
                  value={[config.mode]}
                  variant='outline'
                >
                  <ToggleGroupItem value='chat'>
                    <MessageSquareIcon />
                    <span className='sr-only'>{t('Chat')}</span>
                  </ToggleGroupItem>
                  <ToggleGroupItem value='image'>
                    <SparklesIcon />
                    <span className='sr-only'>{t('Image')}</span>
                  </ToggleGroupItem>
                </ToggleGroup>
                <PlaygroundInputTools
                  config={config}
                  disabled={disabled}
                  hasMessages={hasMessages}
                  onConfigChange={onConfigChange}
                  onClearMessages={onClearMessages}
                  onParameterEnabledChange={onParameterEnabledChange}
                  parameterEnabled={parameterEnabled}
                />
              </div>
            }
          />
        </PromptInputFooter>
      </PromptInput>
    </div>
  )
}
