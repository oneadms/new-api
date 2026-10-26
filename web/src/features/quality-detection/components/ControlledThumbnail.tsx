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
import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { loadThumbnail } from '../lib/artifacts'

export function ControlledThumbnail(props: {
  path: string | null
  label: string
  large?: boolean
}) {
  const { t } = useTranslation()
  const [image, setImage] = useState<{ blob: Blob; url: string } | null>(null)
  const [failedURL, setFailedURL] = useState('')
  const query = useQuery({
    queryKey: ['quality-thumbnail', props.path],
    queryFn: ({ signal }) => loadThumbnail(props.path ?? '', signal),
    enabled: Boolean(props.path),
    retry: false,
    gcTime: 0,
    staleTime: 0,
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
  })
  useEffect(() => {
    if (!query.data) return
    const url = URL.createObjectURL(query.data)
    setImage({ blob: query.data, url })
    return () => URL.revokeObjectURL(url)
  }, [query.data])
  const className = props.large ? 'aspect-[3/2] w-full' : 'h-16 w-28 shrink-0'
  if (!props.path || query.isError || (image && failedURL === image.url)) {
    return (
      <div
        className={`${className} bg-muted text-muted-foreground flex items-center justify-center rounded-lg border p-2 text-center text-xs`}
      >
        {t('Thumbnail unavailable')}
      </div>
    )
  }
  if (!image || image.blob !== query.data || query.isPending) {
    return (
      <div
        role='status'
        aria-label={t('Loading...')}
        className={`${className} bg-muted rounded-lg border`}
      />
    )
  }
  return (
    <img
      src={image.url}
      alt={props.label}
      className={`${className} bg-muted rounded-lg border object-contain`}
      onError={() => setFailedURL(image.url)}
    />
  )
}
