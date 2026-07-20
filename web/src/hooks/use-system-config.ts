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
import { useEffect, useCallback } from 'react'

import { DEFAULT_SYSTEM_NAME, DEFAULT_LOGO } from '@/lib/constants'
import { applyFaviconToDom } from '@/lib/dom-utils'
import {
  useSystemConfigStore,
  type CurrencyConfig,
  type CurrencyDisplayType,
  type MusicPlayerTrack,
  type SystemConfig,
  DEFAULT_CURRENCY_CONFIG,
  DEFAULT_MUSIC_PLAYER_CONFIG,
} from '@/stores/system-config-store'

interface UseSystemConfigOptions {
  /** Automatically fetch config from backend (use only in root component) */
  autoLoad?: boolean
}

interface StatusApiResponse {
  success: boolean
  data: {
    system_name?: string
    logo?: string
    background_image?: string
    content_opacity?: number | string
    footer_html?: string
    demo_site_enabled?: boolean
    display_token_stat_enabled?: boolean
    display_in_currency?: boolean
    quota_display_type?: CurrencyDisplayType
    quota_per_unit?: number
    usd_exchange_rate?: number
    custom_currency_symbol?: string
    custom_currency_exchange_rate?: number
    music_player_enabled?: boolean | string
    music_player_playlist?: string | unknown[]
    music_player_autoplay?: boolean | string
    music_player_show_lyrics?: boolean | string
  }
}

function toNumber(value: unknown, fallback: number): number {
  if (typeof value === 'number' && !Number.isNaN(value)) return value
  if (typeof value === 'string') {
    const parsed = Number(value)
    if (!Number.isNaN(parsed)) return parsed
  }
  return fallback
}

function toBoolean(value: unknown, fallback: boolean): boolean {
  if (typeof value === 'boolean') return value
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (normalized === 'true' || normalized === '1') return true
    if (normalized === 'false' || normalized === '0') return false
  }
  return fallback
}

function toOptionalString(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined
  const trimmed = value.trim()
  return trimmed || undefined
}

function normalizeMusicPlayerTrack(value: unknown): MusicPlayerTrack | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null

  const record = value as Record<string, unknown>
  const url = toOptionalString(record.url)
  if (!url) return null

  return {
    url,
    title: toOptionalString(record.title),
    artist: toOptionalString(record.artist),
    cover: toOptionalString(record.cover),
    lyrics: toOptionalString(record.lyrics),
  }
}

function parseMusicPlayerPlaylist(value: unknown): MusicPlayerTrack[] {
  try {
    let parsed = value
    if (typeof value === 'string') {
      parsed = value.trim() ? JSON.parse(value) : []
    }

    if (!Array.isArray(parsed)) return []

    return parsed
      .map((item) => normalizeMusicPlayerTrack(item))
      .filter((item): item is MusicPlayerTrack => item !== null)
      .slice(0, 50)
  } catch {
    return []
  }
}

/**
 * Map `/api/status` response data to our persisted system config structure
 */
export function mapStatusDataToConfig(
  data: StatusApiResponse['data'] | undefined
): Partial<SystemConfig> {
  if (!data) return {}

  const quotaDisplayType =
    (data.quota_display_type as CurrencyDisplayType | undefined) ??
    DEFAULT_CURRENCY_CONFIG.quotaDisplayType

  const currency: CurrencyConfig = {
    displayInCurrency:
      data.display_in_currency ?? DEFAULT_CURRENCY_CONFIG.displayInCurrency,
    quotaDisplayType,
    quotaPerUnit: toNumber(
      data.quota_per_unit,
      DEFAULT_CURRENCY_CONFIG.quotaPerUnit
    ),
    usdExchangeRate: toNumber(
      data.usd_exchange_rate,
      DEFAULT_CURRENCY_CONFIG.usdExchangeRate
    ),
    customCurrencySymbol:
      data.custom_currency_symbol?.trim() ||
      DEFAULT_CURRENCY_CONFIG.customCurrencySymbol,
    customCurrencyExchangeRate: toNumber(
      data.custom_currency_exchange_rate,
      DEFAULT_CURRENCY_CONFIG.customCurrencyExchangeRate
    ),
  }

  const musicPlayer = {
    enabled: toBoolean(
      data.music_player_enabled,
      DEFAULT_MUSIC_PLAYER_CONFIG.enabled
    ),
    playlist: parseMusicPlayerPlaylist(data.music_player_playlist),
    autoplay: toBoolean(
      data.music_player_autoplay,
      DEFAULT_MUSIC_PLAYER_CONFIG.autoplay
    ),
    showLyrics: toBoolean(
      data.music_player_show_lyrics,
      DEFAULT_MUSIC_PLAYER_CONFIG.showLyrics
    ),
  }

  return {
    systemName: data.system_name || DEFAULT_SYSTEM_NAME,
    logo: data.logo || DEFAULT_LOGO,
    backgroundImage: data.background_image || '',
    contentOpacity: normalizeOpacity(data.content_opacity),
    footerHtml: data.footer_html,
    demoSiteEnabled: data.demo_site_enabled,
    displayTokenStatEnabled: data.display_token_stat_enabled,
    currency,
    musicPlayer,
  }
}

function normalizeOpacity(value: unknown): number {
  const parsed = toNumber(value, 100)
  return Math.min(100, Math.max(0, Math.round(parsed)))
}

function formatCssUrl(value: string): string {
  return `url(${JSON.stringify(value)})`
}

function applySystemBackground(
  backgroundImage: string,
  contentOpacity: number
) {
  if (typeof document === 'undefined') return

  const root = document.documentElement
  const trimmedImage = backgroundImage.trim()
  const opacity = normalizeOpacity(contentOpacity)

  root.style.setProperty('--app-content-bg-percent', `${opacity}%`)

  if (trimmedImage) {
    root.dataset.systemBackground = 'true'
    root.style.setProperty('--app-background-image', formatCssUrl(trimmedImage))
  } else {
    root.removeAttribute('data-system-background')
    root.style.removeProperty('--app-background-image')
  }
}

// Fetch system config from API
async function fetchSystemConfig(): Promise<Partial<SystemConfig>> {
  const response = await fetch('/api/status')
  if (!response.ok) throw new Error('Failed to fetch status')

  const data: StatusApiResponse = await response.json()
  if (!data.success) throw new Error('API returned error')

  return mapStatusDataToConfig(data.data)
}

// Preload image and return cleanup function
function preloadImage(
  src: string,
  onLoad: () => void,
  onError: () => void
): () => void {
  const img = new Image()
  img.onload = onLoad
  img.onerror = onError
  img.src = src

  return () => {
    img.onload = null
    img.onerror = null
  }
}

/**
 * System configuration hook with auto-loading and logo preloading
 *
 * @example
 * // Root component - auto-load from backend
 * useSystemConfig({ autoLoad: true })
 *
 * @example
 * // Other components - use cached config
 * const { systemName, logo, loading } = useSystemConfig()
 */
export function useSystemConfig(options: UseSystemConfigOptions = {}) {
  const { autoLoad = false } = options
  const {
    config,
    loading,
    loadedLogoUrl,
    setConfig,
    setLoadedLogoUrl,
    setLoading,
  } = useSystemConfigStore()

  // Load config from backend
  const loadConfig = useCallback(async () => {
    try {
      setLoading(true)
      const newConfig = await fetchSystemConfig()
      setConfig(newConfig)
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to load system config:', error)
    } finally {
      setLoading(false)
    }
  }, [setConfig, setLoading])

  useEffect(() => {
    if (autoLoad) loadConfig()
  }, [autoLoad, loadConfig])

  useEffect(() => {
    applySystemBackground(config.backgroundImage, config.contentOpacity)
  }, [config.backgroundImage, config.contentOpacity])

  // Preload logo image when URL changes
  useEffect(() => {
    const { logo } = config

    // Skip if logo is already loaded
    if (!logo || logo === loadedLogoUrl) return

    // Preload new logo
    return preloadImage(
      logo,
      () => {
        setLoadedLogoUrl(logo)
        applyFaviconToDom(logo)
      },
      () => {
        if (logo !== DEFAULT_LOGO) {
          // eslint-disable-next-line no-console
          console.error('Failed to load logo:', logo)
        }
        // Mark as loaded even on error to prevent infinite retry
        setLoadedLogoUrl(logo)
      }
    )
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [config.logo, loadedLogoUrl, setLoadedLogoUrl])

  return {
    ...config,
    loading,
    logoLoaded: config.logo === loadedLogoUrl && !!loadedLogoUrl,
  }
}
