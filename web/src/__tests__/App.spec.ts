import { describe, it, expect, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import App from '../App.vue'

describe('App', () => {
  it('renders the chat shell', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.endsWith('/api/conversations') && init?.method === 'POST') {
        return new Response(
          JSON.stringify({
            id: 'conv-1',
            title: 'New chat',
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          }),
        )
      }
      if (url.endsWith('/api/conversations')) {
        return new Response(JSON.stringify({ items: [] }))
      }
      if (url.includes('/messages')) {
        return new Response(JSON.stringify({ items: [] }))
      }
      return new Response('{}')
    })

    const originalFetch = globalThis.fetch
    ;(globalThis as { fetch: typeof fetch }).fetch = fetchMock as unknown as typeof fetch

    try {
      const wrapper = shallowMount(App)
      await Promise.resolve()
      await Promise.resolve()

      expect(wrapper.text()).toContain('New Chat')
    } finally {
      ;(globalThis as { fetch: typeof fetch }).fetch = originalFetch
    }
  })
})
