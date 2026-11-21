import { useTranslation } from 'react-i18next'

import { UserAuthForm } from '@/features/auth/sign-in/components/user-auth-form'

import '@/styles/reseller.css'

export function ResellerLogin() {
  const { t } = useTranslation()
  return (
    <div className='reseller-ledger ledger-login-page'>
      <header className='ledger-masthead'>
        <b className='ledger-brand'>O1Key</b>
      </header>
      <main className='ledger-login-main'>
        <section
          className='ledger-login-panel'
          aria-labelledby='reseller-login-title'
        >
          <p className='ledger-eyebrow'>{t('Reseller center')}</p>
          <h1 id='reseller-login-title' className='ledger-title'>
            {t('Sign in to your account')}
          </h1>
          <UserAuthForm
            className='ledger-auth-form'
            appearance='ledger'
            redirectTo='/reseller/'
          />
        </section>
      </main>
    </div>
  )
}
