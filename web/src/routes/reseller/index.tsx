import { createFileRoute, redirect } from '@tanstack/react-router'

import { ResellerDesk } from '@/features/reseller'
import { resolveAuthentication } from '@/lib/auth-session'

export const Route = createFileRoute('/reseller/')({
  beforeLoad: async () => {
    const result = await resolveAuthentication()
    if (result.kind !== 'authenticated') {
      throw redirect({ to: '/reseller/login', replace: true })
    }
  },
  component: ResellerDesk,
})
