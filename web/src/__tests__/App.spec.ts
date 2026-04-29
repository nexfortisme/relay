import { describe, it, expect, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import App from '../App.vue'
import ConversationSidebar from '../components/ConversationSidebar.vue'

describe('App', () => {
  it('renders the chat shell', async () => {
    const fetchMock = vi.fn<typeof fetch>(async (input: RequestInfo | URL, init?: RequestInit) => {
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
      if (url.includes('/api/conversations')) {
        return new Response(JSON.stringify({ items: [] }))
      }
      if (url.includes('/messages')) {
        return new Response(JSON.stringify({ items: [] }))
      }
      return new Response('{}')
    })

    const originalFetch = globalThis.fetch
    const originalWebSocket = globalThis.WebSocket
    ;(globalThis as { fetch: typeof fetch }).fetch = fetchMock as unknown as typeof fetch
    class MockWebSocket {
      onmessage: ((event: MessageEvent) => void) | null = null
      onerror: ((event: Event) => void) | null = null
      onclose: ((event: CloseEvent) => void) | null = null

      constructor(_url: string) {}

      close = vi.fn<() => void>()
    }
    vi.stubGlobal('WebSocket', MockWebSocket)

    try {
      const wrapper = shallowMount(App, {
        global: {
          plugins: [createPinia()],
        },
      })
      await flushPromises()
      await flushPromises()

      expect(wrapper.findComponent(ConversationSidebar).exists()).toBe(true)
    } finally {
      ;(globalThis as { fetch: typeof fetch }).fetch = originalFetch
      ;(globalThis as { WebSocket: typeof WebSocket }).WebSocket = originalWebSocket
    }
  })
})
