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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  RetailVideoPriceRange,
  RetailVideoPriceTable,
} from '../components/retail-video-prices'
import type { RetailVideoPrices } from '../types'

it('shows all Wan resolutions with the already discounted billable-second price', () => {
  const read = vi
    .spyOn(api, 'get')
    .mockResolvedValue({ data: { data: { groups: [] } } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const prices: RetailVideoPrices = {
    basis: 'billable',
    groups: [
      {
        group: 'wan',
        rows: [
          {
            feature: 'Video generation',
            prices: { '480P': 0.18, '720P': 0.36, '1080P': 0.72 },
          },
        ],
      },
    ],
  }
  try {
    render(
      <QueryClientProvider client={client}>
        <RetailVideoPriceRange prices={prices} />
        <RetailVideoPriceTable prices={prices} />
      </QueryClientProvider>
    )
    expect(
      screen.getByRole('columnheader', { name: '480P' })
    ).toBeInTheDocument()
    expect(
      screen.getByText(/0\.18.*0\.72.*billable second/)
    ).toBeInTheDocument()
    const row = screen.getByRole('row', { name: /Video generation/ })
    expect(within(row).getByText(/0\.18/)).toBeInTheDocument()
    expect(within(row).getByText(/0\.36/)).toBeInTheDocument()
    expect(within(row).getByText(/0\.72/)).toBeInTheDocument()
  } finally {
    read.mockRestore()
    client.clear()
  }
})
