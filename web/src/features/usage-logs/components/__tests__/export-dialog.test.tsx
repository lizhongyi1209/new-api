/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, expect, it } from 'vitest'

import zh from '@/i18n/locales/zh.json'

import { UsageLogsActions } from '../usage-logs-actions'

afterEach(async () => {
  await i18next.changeLanguage('en')
})

it('shows every export option and explanation in Chinese when the interface language is Chinese', async () => {
  i18next.addResourceBundle('zh', 'translation', zh.translation, true, true)
  await i18next.changeLanguage('zh')
  render(
    <QueryClientProvider client={new QueryClient()}>
      <UsageLogsActions isAdmin={false} searchParams={{}} />
    </QueryClientProvider>
  )

  await userEvent.click(
    screen.getByRole('button', { name: i18next.t('Export logs') })
  )
  const dialog = await screen.findByRole('dialog')
  expect(within(dialog).getByText('导出使用日志')).toBeVisible()
  expect(
    within(dialog).getByText('导出筛选后的记录，用于分析或账单对账。')
  ).toBeVisible()
  expect(
    within(dialog).getByText('单次最多导出 31 天、50,000 条记录。')
  ).toBeVisible()
  expect(within(dialog).getByText('导出格式')).toBeVisible()
  expect(within(dialog).getByText('适合电子表格和快速分析')).toBeVisible()
  expect(within(dialog).getByText('完整的结构化日志记录')).toBeVisible()
  expect(within(dialog).getByText('对账 XLSX')).toBeVisible()
  expect(within(dialog).getByText('便于账单对账的格式化报表')).toBeVisible()
  expect(within(dialog).getByPlaceholderText('所有分组')).toBeVisible()
  expect(within(dialog).getByPlaceholderText('所有模型')).toBeVisible()
  expect(within(dialog).getByRole('button', { name: '导出' })).toBeVisible()
})
