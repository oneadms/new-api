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

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'

export function RendererSetupNotice(props: { configured: boolean }) {
  const { t } = useTranslation()

  return (
    <Alert role='status' aria-label={t('Renderer setup')}>
      <AlertTitle>
        {props.configured
          ? t('Renderer is configured; health has not been checked.')
          : t('Renderer configuration is missing or invalid.')}
      </AlertTitle>
      <AlertDescription>
        <p>
          {t(
            'Text and extracted HTML are saved without the renderer. Screenshots require the separate renderer service.'
          )}
        </p>
        <Collapsible>
          <CollapsibleTrigger
            render={
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='h-auto max-w-full py-1 text-left whitespace-normal'
              />
            }
          >
            {t('Renderer setup')}
          </CollapsibleTrigger>
          <CollapsibleContent
            role='region'
            aria-label={t('Renderer setup')}
            className='pt-3 [overflow-wrap:anywhere]'
          >
            <ol className='list-decimal space-y-2 ps-5'>
              <li>
                {t(
                  'Deploy the isolated renderer from {{path}} using its published image. Keep the browser worker offline and do not publish host ports.',
                  { path: 'tools/quality-renderer/README.md' }
                )}
              </li>
              <li>
                {t(
                  'Connect only the application and renderer ingress to the private {{network}} network.',
                  { network: 'new-api-quality-renderer-ingress' }
                )}
              </li>
              <li>
                {t(
                  'Set {{urlVariable}} to {{url}} in the application environment. Use the base URL without /render.',
                  {
                    urlVariable: 'QUALITY_RENDERER_URL',
                    url: 'http://quality-renderer:3000',
                  }
                )}
              </li>
              <li>
                {t(
                  'Set the same dedicated {{tokenVariable}} in the application and renderer ingress environments only. Keep it out of the browser worker and frontend.',
                  { tokenVariable: 'QUALITY_RENDERER_TOKEN' }
                )}
              </li>
              <li>
                {t(
                  'Recreate the affected containers after changing environment variables. Verify worker health and an authenticated test screenshot on the target Linux host; configuration alone does not prove readiness.'
                )}
              </li>
            </ol>
          </CollapsibleContent>
        </Collapsible>
      </AlertDescription>
    </Alert>
  )
}
