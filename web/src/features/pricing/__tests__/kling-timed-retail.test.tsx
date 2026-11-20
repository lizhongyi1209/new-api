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

it('labels the billable and output units and excludes HDR add-ons from the SDR card price', () => {
  const apiRead = vi
    .spyOn(api, 'get')
    .mockResolvedValue({ data: { data: { groups: [] } } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const prices: RetailVideoPrices = {
    basis: 'timed',
    groups: [
      {
        group: 'official',
        rows: [
          {
            feature: 'No input video · per output second',
            prices: { '720P': 0.9, '1080P': 1.35, '4K': 3.15 },
          },
          {
            feature: 'HDR add-on without input video · per output second',
            prices: { '1080P': 0.27, '4K': 0.45 },
          },
          {
            feature: 'With input video · per billable second',
            prices: { '720P': 0.54, '1080P': 0.81, '4K': 1.89 },
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
    expect(screen.getByText(/0\.9.*3\.15.*SDR/)).toBeInTheDocument()
    const row = screen.getByRole('row', {
      name: /HDR add-on without input video/,
    })
    expect(within(row).getByText('—')).toBeInTheDocument()
    expect(within(row).getByText(/0\.45/)).toBeInTheDocument()
    expect(
      screen.getByRole('row', { name: /per billable second/ })
    ).toBeInTheDocument()
  } finally {
    apiRead.mockRestore()
    client.clear()
  }
})
