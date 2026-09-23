import { afterEach, expect, test, vi } from 'vitest'

import { bootstrapAuthentication } from '@/lib/auth-session'
import { Route as SignInRoute } from '@/routes/(auth)/sign-in'
import { Route as AuthenticatedRoute } from '@/routes/_authenticated/route'
import type { AuthBundle } from '@/stores/auth-store'

vi.mock('@/lib/auth-session', () => ({ bootstrapAuthentication: vi.fn() }))
vi.mock('@/features/auth/sign-in', () => ({ SignIn: () => null }))
vi.mock('@/components/layout', () => ({ AuthenticatedLayout: () => null }))

afterEach(() => vi.resetAllMocks())

const bundle: AuthBundle = {
  access_token: 'synthetic-token',
  token_type: 'Bearer',
  access_expires_at: 1_900_000_000,
  user: { id: 1, username: 'test-user', role: 1 },
  session: {
    sid: 'synthetic-session',
    current: true,
    login_method: 'password',
    ip: '127.0.0.1',
    user_agent: 'test',
    created_at: 1,
    last_active_at: 1,
    expires_at: 1_900_000_000,
  },
}
const signInGuard = SignInRoute.options.beforeLoad as unknown as (context: {
  search: { redirect?: string }
}) => Promise<void>
const authenticatedGuard = AuthenticatedRoute.options
  .beforeLoad as unknown as (context: {
  location: { href: string }
}) => Promise<void>

test('anonymous sign-in stays available after the server session check', async () => {
  vi.mocked(bootstrapAuthentication).mockResolvedValue({ kind: 'anonymous' })
  await expect(signInGuard({ search: {} })).resolves.toBeUndefined()
  expect(bootstrapAuthentication).toHaveBeenCalledTimes(1)
})

test('authenticated sign-in rejects an external return address', async () => {
  vi.mocked(bootstrapAuthentication).mockResolvedValue({
    kind: 'authenticated',
    bundle,
  })
  await expect(
    signInGuard({ search: { redirect: 'https://evil.example/credentials' } })
  ).rejects.toMatchObject({
    options: { href: '/dashboard', replace: true },
  })
})

test('a protected route rejects an anonymous session and retains the return address', async () => {
  vi.mocked(bootstrapAuthentication).mockResolvedValue({ kind: 'anonymous' })
  await expect(
    authenticatedGuard({ location: { href: '/wallet?tab=history' } })
  ).rejects.toMatchObject({
    options: { to: '/sign-in', search: { redirect: '/wallet?tab=history' } },
  })
})

test('a confirmed session opens a protected route', async () => {
  vi.mocked(bootstrapAuthentication).mockResolvedValue({
    kind: 'authenticated',
    bundle,
  })
  await expect(
    authenticatedGuard({ location: { href: '/wallet' } })
  ).resolves.toBeUndefined()
})
