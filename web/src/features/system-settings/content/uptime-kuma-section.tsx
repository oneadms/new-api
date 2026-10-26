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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { SettingsSection } from '../components/settings-section'

export function UptimeKumaSection() {
  const { t } = useTranslation()
  return (
    <SettingsSection title={t('Service Status')}>
      <div className='space-y-4 p-4'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Configure native status publishing, channel samples and detection schedules in the quality detection workspace.'
          )}
        </p>
        <Button render={<Link to='/quality-detection' />} nativeButton={false}>
          {t('Quality detection')}
        </Button>
      </div>
    </SettingsSection>
  )
}
