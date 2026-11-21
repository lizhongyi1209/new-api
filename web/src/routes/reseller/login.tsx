import { createFileRoute, redirect } from '@tanstack/react-router'

import { ResellerLogin } from '@/features/reseller/login'
import { resolveAuthentication } from '@/lib/auth-session'

export const Route = createFileRoute('/reseller/login')({
  beforeLoad: async () => {
    const result = await resolveAuthentication()
    if (result.kind === 'authenticated') {
      throw redirect({ to: '/reseller', replace: true })
    }
  },
  component: ResellerLogin,
})
