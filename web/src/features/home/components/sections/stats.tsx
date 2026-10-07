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
import { useTranslation } from 'react-i18next'

export function Stats() {
  const { t } = useTranslation()
  return (
    <section
      className='home-providers'
      aria-label={t('Multi-protocol Compatible')}
    >
      <div className='home-container'>
        <p className='home-eyebrow'>
          {t('Many possibilities. One familiar API.')}
        </p>
        <div className='home-provider-names' aria-label={t('Models')}>
          <span>OpenAI</span>
          <span>
            Claude
            <span className='provider-asterisk' aria-hidden='true'>
              ✳
            </span>
          </span>
          <span>
            Gemini
            <span className='provider-spark' aria-hidden='true'>
              ✦
            </span>
          </span>
          <span>DeepSeek</span>
          <span>Qwen</span>
          <span>Llama</span>
        </div>
      </div>
    </section>
  )
}
