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
import { Plus, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { useAigcElements } from './aigc-elements-provider'

export function AigcElementsPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen, showAll, setShowAll } = useAigcElements()
  return (
    <div className='flex gap-2'>
      <Button
        size='sm'
        variant={showAll ? 'default' : 'outline'}
        onClick={() => setShowAll((prev) => !prev)}
        title={t('Toggle between your subjects and all users subjects')}
      >
        <Users className='h-4 w-4' />
        {showAll ? t('All Users') : t('My Subjects')}
      </Button>
      <Button size='sm' onClick={() => setOpen('create')}>
        <Plus className='h-4 w-4' />
        {t('Create Subject')}
      </Button>
    </div>
  )
}
