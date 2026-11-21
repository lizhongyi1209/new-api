import {
  cleanup,
  render,
  screen,
  fireEvent,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'

import { dayRange } from '../period'
import { RangeControl } from '../range-control'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))
afterEach(cleanup)

it('keeps the applied range when the user cancels a draft', async () => {
  const user = userEvent.setup()
  const change = vi.fn()
  render(<RangeControl value={dayRange('2026-01-10')} onChange={change} />)
  await user.click(screen.getByRole('button', { name: '2026-01-10' }))
  fireEvent.change(screen.getByLabelText('Start date'), {
    target: { value: '2026-01-01' },
  })
  await user.click(screen.getByRole('button', { name: 'Cancel' }))
  expect(change).not.toHaveBeenCalled()
  await user.click(screen.getByRole('button', { name: '2026-01-10' }))
  expect((screen.getByLabelText('Start date') as HTMLInputElement).value).toBe(
    '2026-01-10'
  )
})

it('rejects a reversed date range and applies a valid exact time range', async () => {
  const user = userEvent.setup()
  const change = vi.fn()
  render(<RangeControl value={dayRange('2026-01-10')} onChange={change} />)
  await user.click(screen.getByRole('button', { name: '2026-01-10' }))
  fireEvent.change(screen.getByLabelText('Start date'), {
    target: { value: '2026-01-11' },
  })
  await user.click(screen.getByRole('button', { name: 'Apply' }))
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Invalid date range'
  )
  expect(change).not.toHaveBeenCalled()
  fireEvent.change(screen.getByLabelText('Start date'), {
    target: { value: '2026-01-09' },
  })
  await user.click(screen.getByRole('checkbox', { name: 'Exact time' }))
  fireEvent.change(screen.getByLabelText('Start time'), {
    target: { value: '09:30:00' },
  })
  await user.click(screen.getByRole('button', { name: 'Apply' }))
  await waitFor(() =>
    expect(change).toHaveBeenCalledWith(
      {
        from: '2026-01-09',
        to: '2026-01-10',
        start: '09:30:00',
        end: '23:59:59',
        precise: true,
      },
      false
    )
  )
})
