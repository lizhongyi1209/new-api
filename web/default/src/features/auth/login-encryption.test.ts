import {
  constants,
  createDecipheriv,
  generateKeyPairSync,
  privateDecrypt,
  webcrypto,
} from 'node:crypto'

import { afterEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { login } from './api'
import { clearPasswordEncryptionCache } from './lib/password-encryption'

const { publicKey, privateKey } = generateKeyPairSync('rsa', {
  modulusLength: 2048,
})
const publicPEM = publicKey.export({ type: 'spki', format: 'pem' }).toString()

afterEach(() => {
  clearPasswordEncryptionCache()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('password login wire contract', () => {
  test.each([
    ['short password', 'unchanged secret 🔒', true],
    ['long Unicode password', '🔒'.repeat(128), true],
    ['HTTP compatibility encryption', '🔒'.repeat(128), false],
  ])(
    'encrypts %s without sending plaintext',
    async (_name, password, subtle) => {
      vi.stubGlobal('crypto', subtle ? webcrypto : { subtle: undefined })
      vi.spyOn(api, 'get').mockResolvedValue({
        data: {
          success: true,
          data: { kid: 'test-key', public_key: publicPEM },
        },
      })
      const post = vi.spyOn(api, 'post').mockResolvedValue({
        data: { success: true },
      })

      await login({
        username: 'test-user',
        password,
        passwordEncryptionEnabled: true,
        turnstile: 'token&extra=bad',
      })

      expect(post).toHaveBeenCalledTimes(1)
      const [url, body, config] = post.mock.calls[0]
      expect(url).toBe('/api/user/login')
      expect(config).toMatchObject({
        params: { turnstile: 'token&extra=bad' },
        skipAuthRefresh: true,
      })
      expect(body).not.toHaveProperty('password')
      expect(body).toMatchObject({
        username: 'test-user',
        encryption_key_id: 'test-key',
      })
      const ciphertext = (body as { password_encrypted: string })
        .password_encrypted
      const rsaOptions = {
        key: privateKey,
        padding: constants.RSA_PKCS1_OAEP_PADDING,
        oaepHash: 'sha256',
      }
      let plaintext: Buffer
      if (ciphertext.startsWith('v2.')) {
        const [, wrapped, nonce, payload] = ciphertext.split('.')
        const secret = privateDecrypt(
          { ...rsaOptions, oaepLabel: Buffer.from('password-v2') },
          Buffer.from(wrapped, 'base64')
        )
        const encrypted = Buffer.from(payload, 'base64')
        const decipher = createDecipheriv(
          'aes-256-gcm',
          secret,
          Buffer.from(nonce, 'base64')
        )
        decipher.setAAD(Buffer.from('password-v2:test-key'))
        decipher.setAuthTag(encrypted.subarray(-16))
        plaintext = Buffer.concat([
          decipher.update(encrypted.subarray(0, -16)),
          decipher.final(),
        ])
      } else {
        plaintext = privateDecrypt(
          rsaOptions,
          Buffer.from(ciphertext, 'base64')
        )
      }
      expect(plaintext.toString('utf8')).toBe(password)
    }
  )

  test('uses the existing unencrypted contract only when encryption is disabled', async () => {
    const get = vi.spyOn(api, 'get')
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { success: true } })
    await login({
      username: 'test-user',
      password: 'legacy secret',
      passwordEncryptionEnabled: false,
    })
    expect(get).not.toHaveBeenCalled()
    expect(post).toHaveBeenCalledWith(
      '/api/user/login',
      {
        username: 'test-user',
        password: 'legacy secret',
      },
      expect.objectContaining({ skipAuthRefresh: true })
    )
  })

  test('does not submit credentials or fall back when the key is unavailable', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({ data: { success: false } })
    const post = vi.spyOn(api, 'post')
    await expect(
      login({
        username: 'test-user',
        password: 'secret',
        passwordEncryptionEnabled: true,
      })
    ).rejects.toThrow('Login failed')
    expect(post).not.toHaveBeenCalled()
  })

  test.each(['business failure', 'transport failure'])(
    'reloads a cached key after %s',
    async (failure) => {
      vi.stubGlobal('crypto', webcrypto)
      const get = vi.spyOn(api, 'get').mockResolvedValue({
        data: {
          success: true,
          data: { kid: 'test-key', public_key: publicPEM },
        },
      })
      const post = vi
        .spyOn(api, 'post')
        .mockResolvedValue({ data: { success: true } })
      if (failure === 'business failure') {
        post.mockResolvedValueOnce({ data: { success: false } })
      } else {
        post.mockRejectedValueOnce(new Error('offline'))
      }
      const payload = {
        username: 'test-user',
        password: 'secret',
        passwordEncryptionEnabled: true,
      }
      if (failure === 'business failure') {
        expect(await login(payload)).toEqual({ success: false })
      } else {
        await expect(login(payload)).rejects.toThrow('offline')
      }
      await login(payload)
      expect(get).toHaveBeenCalledTimes(2)
      expect(
        post.mock.calls.every(
          ([, body]) => !Object.hasOwn(body as object, 'password')
        )
      ).toBe(true)
    }
  )
})
