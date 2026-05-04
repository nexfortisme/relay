import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import ChatView from '../views/ChatView.vue'
import { useAppStore } from '../stores/appStore'

vi.mock('../lib/api', () => ({
  archiveConversation: vi.fn<() => void>(),
  conversationStreamUrl: vi.fn<(conversationId: string) => string>(() => 'ws://localhost/stream'),
  createConversation: vi.fn<() => void>(),
  createFailedMessage: vi.fn<() => void>(),
  createMessage: vi.fn<() => void>(),
  deleteConversation: vi.fn<() => void>(),
  fileDownloadUrl: vi.fn<(fileId: string) => string>((fileId) => `/files/${fileId}`),
  getSettings: vi.fn<() => void>(),
  listConversations: vi.fn<() => Promise<never[]>>(async () => []),
  listMessages: vi.fn<() => Promise<never[]>>(async () => []),
  renameConversation: vi.fn<() => void>(),
  requeueMessage: vi.fn<() => void>(),
  restoreConversation: vi.fn<() => void>(),
  stopConversationGeneration: vi.fn<() => void>(),
  suggestConversationTitle: vi.fn<() => void>(),
  updateSettings: vi.fn<() => void>(),
}))

class MockWebSocket {
  static instances: MockWebSocket[] = []

  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null

  constructor(_url: string) {
    MockWebSocket.instances.push(this)
  }

  close = vi.fn<() => void>()
}

describe('ChatView stream lifecycle', () => {
  beforeEach(() => {
    MockWebSocket.instances = []
    vi.stubGlobal('WebSocket', MockWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('leaves the selected chat stream open when the chat view unmounts', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/chat', component: ChatView }],
    })
    await router.push('/chat')
    await router.isReady()

    const store = useAppStore()
    await store.selectConversation('conv-1')
    const socket = MockWebSocket.instances[0]
    if (!socket) {
      throw new Error('expected stream websocket to be created')
    }

    const wrapper = mount(ChatView, {
      global: {
        plugins: [pinia, router],
        stubs: {
          ChatComposer: true,
          ChatHeader: true,
          ConversationSidebar: true,
          EmptyChatGreeting: true,
          MessageList: true,
          PageNavTabs: true,
        },
      },
    })

    expect(MockWebSocket.instances).toHaveLength(1)

    wrapper.unmount()

    expect(socket.close).not.toHaveBeenCalled()
  })
})
