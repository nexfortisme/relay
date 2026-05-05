import { describe, it, expect, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import App from '../App.vue'
import ChatView from '../views/ChatView.vue'
import HomeView from '../views/HomeView.vue'
import ConversationSidebar from '../components/ConversationSidebar.vue'

describe('App', () => {
  it('renders the chat shell on /chat', async () => {
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

    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/home', component: HomeView },
        { path: '/chat', component: ChatView },
      ],
    })
    await router.push('/chat')
    await router.isReady()

    try {
      const wrapper = mount(App, {
        global: {
          plugins: [createPinia(), router],
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
