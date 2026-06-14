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
import { useState } from 'react'
import {
  PaperclipIcon,
  FileIcon,
  ImageIcon,
  ScreenShareIcon,
  CameraIcon,
  GlobeIcon,
  SendIcon,
  SquareIcon,
  BarChartIcon,
  BoxIcon,
  NotepadTextIcon,
  CodeSquareIcon,
  GraduationCapIcon,
  MessageSquareIcon,
  SparklesIcon,
  PlusIcon,
  XIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
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
import {
  PromptInput,
  PromptInputButton,
  PromptInputFooter,
  PromptInputTextarea,
  PromptInputTools,
  type PromptInputMessage,
} from '@/components/ai-elements/prompt-input'
import { Suggestion, Suggestions } from '@/components/ai-elements/suggestion'
import { ModelGroupSelector } from '@/components/model-group-selector'
import type {
  ModelOption,
  GroupOption,
  ImageResponseFormat,
  PlaygroundMode,
  PlaygroundSubmitInput,
} from '../types'

interface PlaygroundInputProps {
  onSubmit: (input: PlaygroundSubmitInput) => void
  onStop?: () => void
  disabled?: boolean
  isGenerating?: boolean
  mode: PlaygroundMode
  onModeChange: (value: PlaygroundMode) => void
  models: ModelOption[]
  modelValue: string
  onModelChange: (value: string) => void
  isModelLoading?: boolean
  groups: GroupOption[]
  groupValue: string
  onGroupChange: (value: string) => void
  imageSize: string
  onImageSizeChange: (value: string) => void
  imageQuality: string
  onImageQualityChange: (value: string) => void
  imageResponseFormat: ImageResponseFormat
  onImageResponseFormatChange: (value: ImageResponseFormat) => void
  imageCount: number
  onImageCountChange: (value: number) => void
}

const suggestions = [
  { icon: BarChartIcon, text: 'Analyze data', color: '#76d0eb' },
  { icon: BoxIcon, text: 'Surprise me', color: '#76d0eb' },
  { icon: NotepadTextIcon, text: 'Summarize text', color: '#ea8444' },
  { icon: CodeSquareIcon, text: 'Code', color: '#6c71ff' },
  { icon: GraduationCapIcon, text: 'Get advice', color: '#76d0eb' },
  { icon: null, text: 'More' },
]

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

const imageCountOptions = [1, 2, 3, 4]

export function PlaygroundInput({
  onSubmit,
  onStop,
  disabled,
  isGenerating,
  mode,
  onModeChange,
  models,
  modelValue,
  onModelChange,
  isModelLoading = false,
  groups,
  groupValue,
  onGroupChange,
  imageSize,
  onImageSizeChange,
  imageQuality,
  onImageQualityChange,
  imageResponseFormat,
  onImageResponseFormatChange,
  imageCount,
  onImageCountChange,
}: PlaygroundInputProps) {
  const { t } = useTranslation()
  const [text, setText] = useState('')
  const [imageReferenceUrls, setImageReferenceUrls] = useState(['', ''])

  const isModelSelectDisabled =
    disabled || isModelLoading || models.length === 0
  const isGroupSelectDisabled = disabled || groups.length === 0
  const isImageMode = mode === 'image'

  const getImageQualityLabel = (value: string) => {
    switch (value) {
      case 'auto':
        return t('Auto')
      case 'standard':
        return t('Standard')
      case 'hd':
        return t('HD')
      case 'low':
        return t('Low')
      case 'medium':
        return t('Medium')
      case 'high':
        return t('High')
      default:
        return value
    }
  }

  const getImageResponseFormatLabel = (value: ImageResponseFormat) => {
    switch (value) {
      case 'auto':
        return t('Auto')
      case 'url':
        return t('URL')
      case 'b64_json':
        return t('Base64 JSON')
    }
  }

  const getImageReferencePlaceholder = (index: number) => {
    switch (index) {
      case 0:
        return t('Image URL')
      case 1:
        return t('Image2 URL')
      case 2:
        return t('Image3 URL')
      default:
        return t('Image URL')
    }
  }

  const handleSubmit = (message: PromptInputMessage) => {
    if (!message.text?.trim() || disabled) return
    onSubmit({
      text: message.text,
      imageReferenceUrls: isImageMode ? imageReferenceUrls : undefined,
    })
    setText('')
  }

  const handleFileAction = (action: string) => {
    toast.info(t('Feature in development'), {
      description: action,
    })
  }

  const handleSuggestionClick = (suggestion: string) => {
    onSubmit({ text: suggestion })
  }

  const updateImageReferenceUrl = (index: number, value: string) => {
    setImageReferenceUrls((prev) =>
      prev.map((url, currentIndex) => (currentIndex === index ? value : url))
    )
  }

  const addImageReferenceUrl = () => {
    setImageReferenceUrls((prev) => (prev.length >= 3 ? prev : [...prev, '']))
  }

  const removeImageReferenceUrl = (index: number) => {
    setImageReferenceUrls((prev) =>
      prev.length <= 1
        ? ['']
        : prev.filter((_, currentIndex) => currentIndex !== index)
    )
  }

  return (
    <div className='grid shrink-0 gap-4 px-1 md:pb-4'>
      <PromptInput groupClassName='rounded-xl' onSubmit={handleSubmit}>
        <PromptInputTextarea
          autoComplete='off'
          autoCorrect='off'
          autoCapitalize='off'
          spellCheck={false}
          className='px-5 md:text-base'
          disabled={disabled}
          onChange={(event) => setText(event.target.value)}
          placeholder={
            isImageMode ? t('Describe an image to generate') : t('Ask anything')
          }
          value={text}
        />

        {isImageMode && (
          <div className='border-border/70 flex flex-col gap-3 border-t px-3 py-3'>
            <div className='grid gap-2 sm:grid-cols-4'>
              <div className='flex flex-col gap-1.5'>
                <span className='text-muted-foreground text-xs'>
                  {t('Size')}
                </span>
                <Select
                  items={imageSizeOptions.map((value) => ({
                    value,
                    label: value,
                  }))}
                  value={imageSize}
                  onValueChange={(value) =>
                    value !== null && onImageSizeChange(value)
                  }
                >
                  <SelectTrigger className='w-full'>
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
              </div>

              <div className='flex flex-col gap-1.5'>
                <span className='text-muted-foreground text-xs'>
                  {t('Quality')}
                </span>
                <Select
                  items={imageQualityOptions.map((value) => ({
                    value,
                    label: getImageQualityLabel(value),
                  }))}
                  value={imageQuality}
                  onValueChange={(value) =>
                    value !== null && onImageQualityChange(value)
                  }
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {imageQualityOptions.map((value) => (
                        <SelectItem key={value} value={value}>
                          {getImageQualityLabel(value)}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>

              <div className='flex flex-col gap-1.5'>
                <span className='text-muted-foreground text-xs'>
                  {t('Response')}
                </span>
                <Select
                  items={imageResponseFormatOptions.map((value) => ({
                    value,
                    label: getImageResponseFormatLabel(value),
                  }))}
                  value={imageResponseFormat}
                  onValueChange={(value) =>
                    value !== null &&
                    onImageResponseFormatChange(value as ImageResponseFormat)
                  }
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {imageResponseFormatOptions.map((value) => (
                        <SelectItem key={value} value={value}>
                          {getImageResponseFormatLabel(value)}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>

              <div className='flex flex-col gap-1.5'>
                <span className='text-muted-foreground text-xs'>
                  {t('Count')}
                </span>
                <Select
                  items={imageCountOptions.map((value) => ({
                    value: `${value}`,
                    label: `${value}`,
                  }))}
                  value={`${imageCount}`}
                  onValueChange={(value) =>
                    value !== null && onImageCountChange(Number(value))
                  }
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      {imageCountOptions.map((value) => (
                        <SelectItem key={value} value={`${value}`}>
                          {value}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className='flex flex-col gap-2'>
              {imageReferenceUrls.map((url, index) => (
                <div className='flex items-center gap-2' key={index}>
                  <Input
                    disabled={disabled}
                    onChange={(event) =>
                      updateImageReferenceUrl(index, event.target.value)
                    }
                    placeholder={getImageReferencePlaceholder(index)}
                    value={url}
                  />
                  <PromptInputButton
                    aria-label={t('Remove reference image')}
                    disabled={disabled || imageReferenceUrls.length <= 1}
                    onClick={() => removeImageReferenceUrl(index)}
                    type='button'
                    variant='ghost'
                  >
                    <XIcon />
                  </PromptInputButton>
                </div>
              ))}

              {imageReferenceUrls.length < 3 && (
                <PromptInputButton
                  className='w-fit'
                  disabled={disabled}
                  onClick={addImageReferenceUrl}
                  type='button'
                  variant='outline'
                >
                  <PlusIcon data-icon='inline-start' />
                  {t('Add reference image')}
                </PromptInputButton>
              )}
            </div>
          </div>
        )}

        <PromptInputFooter className='p-2.5'>
          <PromptInputTools>
            <ToggleGroup
              value={[mode]}
              onValueChange={(value) => {
                const nextMode = value[0]
                if (nextMode === 'chat' || nextMode === 'image') {
                  onModeChange(nextMode)
                }
              }}
              disabled={disabled}
              size='sm'
              variant='outline'
            >
              <ToggleGroupItem value='chat'>
                <MessageSquareIcon data-icon='inline-start' />
                <span className='hidden sm:inline'>{t('Chat')}</span>
                <span className='sr-only sm:hidden'>{t('Chat')}</span>
              </ToggleGroupItem>
              <ToggleGroupItem value='image'>
                <SparklesIcon data-icon='inline-start' />
                <span className='hidden sm:inline'>{t('Image')}</span>
                <span className='sr-only sm:hidden'>{t('Image')}</span>
              </ToggleGroupItem>
            </ToggleGroup>

            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <PromptInputButton
                    className='border font-medium'
                    disabled={disabled}
                    variant='outline'
                  />
                }
              >
                <PaperclipIcon size={16} />
                <span className='hidden sm:inline'>{t('Attach')}</span>
                <span className='sr-only sm:hidden'>{t('Attach')}</span>
              </DropdownMenuTrigger>
              <DropdownMenuContent align='start'>
                <DropdownMenuItem
                  onClick={() => handleFileAction('upload-file')}
                >
                  <FileIcon className='mr-2' size={16} />
                  {t('Upload file')}
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => handleFileAction('upload-photo')}
                >
                  <ImageIcon className='mr-2' size={16} />
                  {t('Upload photo')}
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => handleFileAction('take-screenshot')}
                >
                  <ScreenShareIcon className='mr-2' size={16} />
                  {t('Take screenshot')}
                </DropdownMenuItem>
                <DropdownMenuItem
                  onClick={() => handleFileAction('take-photo')}
                >
                  <CameraIcon className='mr-2' size={16} />
                  {t('Take photo')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            <PromptInputButton
              className='border font-medium'
              disabled={disabled}
              onClick={() => toast.info(t('Search feature in development'))}
              variant='outline'
            >
              <GlobeIcon size={16} />
              <span className='hidden sm:inline'>{t('Search')}</span>
              <span className='sr-only sm:hidden'>{t('Search')}</span>
            </PromptInputButton>
          </PromptInputTools>

          <div className='flex items-center gap-1.5 md:gap-2'>
            <ModelGroupSelector
              selectedModel={modelValue}
              models={models}
              onModelChange={onModelChange}
              selectedGroup={groupValue}
              groups={groups}
              onGroupChange={onGroupChange}
              disabled={isModelSelectDisabled || isGroupSelectDisabled}
            />

            {isGenerating && onStop ? (
              <PromptInputButton
                className='text-foreground font-medium'
                onClick={onStop}
                variant='secondary'
              >
                <SquareIcon className='fill-current' size={16} />
                <span className='hidden sm:inline'>{t('Stop')}</span>
                <span className='sr-only sm:hidden'>{t('Stop')}</span>
              </PromptInputButton>
            ) : (
              <PromptInputButton
                className='text-foreground font-medium'
                disabled={disabled || !text.trim()}
                type='submit'
                variant='secondary'
              >
                <SendIcon size={16} />
                <span className='hidden sm:inline'>
                  {isImageMode ? t('Generate') : t('Send')}
                </span>
                <span className='sr-only sm:hidden'>
                  {isImageMode ? t('Generate') : t('Send')}
                </span>
              </PromptInputButton>
            )}
          </div>
        </PromptInputFooter>
      </PromptInput>

      {!isImageMode && (
        <Suggestions>
          {suggestions.map(({ icon: Icon, text, color }) => (
            <Suggestion
              className={`text-xs font-normal sm:text-sm ${
                text === 'More' ? 'hidden sm:flex' : ''
              }`}
              key={text}
              onClick={() => handleSuggestionClick(text)}
              suggestion={text}
            >
              {Icon && <Icon size={16} style={{ color }} />}
              {text}
            </Suggestion>
          ))}
        </Suggestions>
      )}
    </div>
  )
}
