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
import { describe, expect, test } from 'vitest'

import {
  formatByteSize,
  formatDurationMilliseconds,
  formatTransferSpeed,
} from '../format'

describe('generate image timing display', () => {
  test('keeps millisecond precision across display units', () => {
    expect(formatDurationMilliseconds(12.345)).toBe('12.345 ms')
    expect(formatDurationMilliseconds(31_842.125)).toBe('31.842 s')
    expect(formatDurationMilliseconds(62_345)).toBe('1m 2.345s')
  })

  test('formats byte counts and transfer speed for the timing table', () => {
    expect(formatByteSize(1_048_576)).toBe('1.00 MiB')
    expect(formatTransferSpeed(1_000_000, 1000)).toBe('8.00 Mbps')
  })

  test('uses a dash when a metric cannot be observed', () => {
    expect(formatDurationMilliseconds(undefined)).toBe('—')
    expect(formatByteSize(undefined)).toBe('—')
    expect(formatTransferSpeed(0, 1000)).toBe('—')
  })
})
