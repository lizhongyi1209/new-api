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
import { render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { ModelPerfBadge } from './model-perf-badge'

const hour = Date.UTC(2026, 8, 13, 0) / 1000

afterEach(() => vi.useRealTimers())

test('anchors hourly health to the server snapshot despite client clock skew and distinguishes failures from missing traffic', () => {
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-09-14T10:00:00Z'))
  render(
    <ModelPerfBadge
      perf={{
        avg_latency_ms: 100,
        avg_tps: 42,
        success_rate: 90,
        hourly_window_end_ts: hour + 1800,
        recent_success_series: [
          { ts: hour - 23 * 3600, success_rate: 100 },
          { ts: hour - 3600, success_rate: 0 },
          { ts: hour, success_rate: null },
          { ts: hour + 3600, success_rate: 100 },
          { ts: hour - 2 * 3600, success_rate: 150 },
        ],
      }}
    />
  )
  const strip = screen.getByRole('img')
  expect(strip.children).toHaveLength(24)
  expect(screen.getByTitle(/: 100\.0%$/)).toHaveClass('bg-emerald-500')
  expect(screen.getByTitle(/: 0\.0%$/)).toHaveClass('bg-red-500')
  expect(screen.getAllByTitle(/: No data$/)).toHaveLength(22)
  expect(screen.getByText('90.0%')).toBeInTheDocument()
})

test('does not stretch the old three samples into hourly health and keeps details reachable without metrics', () => {
  const { rerender } = render(
    <ModelPerfBadge
      perf={{
        avg_latency_ms: 100,
        avg_tps: 1,
        success_rate: 100,
        recent_success_rates: [100, 100, 100],
      }}
    />
  )
  expect(screen.getAllByTitle(/: No data$/)).toHaveLength(24)
  rerender(
    <ModelPerfBadge perf={undefined}>
      <button type='button'>Details</button>
    </ModelPerfBadge>
  )
  expect(screen.getByRole('button', { name: 'Details' })).toBeInTheDocument()
  expect(screen.getByText('—%')).toBeInTheDocument()
  expect(screen.getAllByTitle(/: No data$/)).toHaveLength(24)
})
