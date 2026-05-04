import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from '../stores/appStore'

vi.mock('../lib/api', () => ({
  archiveConversation: vi.fn<() => void>(),
  conversationStreamUrl: vi.fn<(conversationId: string) => string>(() => 'ws://localhost/stream'),
  createConversation: vi.fn<() => void>(),
  createFailedMessage: vi.fn<() => void>(),
  createMessage: vi.fn<() => void>(),
  deleteConversation: vi.fn<() => void>(),
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

function emit(socket: MockWebSocket, payload: unknown) {
  socket.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent)
}

describe('appStore streaming', () => {
  let flushAnimationFrame: FrameRequestCallback | null = null
  let originalWebSocket: typeof WebSocket
  let originalRequestAnimationFrame: typeof window.requestAnimationFrame
  let originalCancelAnimationFrame: typeof window.cancelAnimationFrame

  beforeEach(() => {
    setActivePinia(createPinia())
    MockWebSocket.instances = []
    flushAnimationFrame = null
    originalWebSocket = globalThis.WebSocket
    originalRequestAnimationFrame = window.requestAnimationFrame
    originalCancelAnimationFrame = window.cancelAnimationFrame
    vi.stubGlobal('WebSocket', MockWebSocket)
    Object.defineProperty(window, 'requestAnimationFrame', {
      configurable: true,
      value: vi.fn<(callback: FrameRequestCallback) => number>((callback) => {
        flushAnimationFrame = callback
        return 1
      }),
    })
    Object.defineProperty(window, 'cancelAnimationFrame', {
      configurable: true,
      value: vi.fn<(handle: number) => void>(),
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    Object.defineProperty(window, 'requestAnimationFrame', {
      configurable: true,
      value: originalRequestAnimationFrame,
    })
    Object.defineProperty(window, 'cancelAnimationFrame', {
      configurable: true,
      value: originalCancelAnimationFrame,
    })
    globalThis.WebSocket = originalWebSocket
  })

  it('batches token and thinking websocket deltas into one visible update', async () => {
    const store = useAppStore()
    await store.selectConversation('conv-1')
    const socket = MockWebSocket.instances[0]
    if (!socket) {
      throw new Error('expected stream websocket to be created')
    }

    emit(socket, { type: 'token', messageId: 'msg-1', token: 'Hel' })
    emit(socket, { type: 'token', messageId: 'msg-1', token: 'lo' })
    emit(socket, { type: 'thinking', messageId: 'msg-1', thinking: 'Plan' })
    emit(socket, { type: 'thinking', messageId: 'msg-1', thinking: 'ning' })

    expect(store.messages).toEqual([])
    expect(window.requestAnimationFrame).toHaveBeenCalledTimes(1)

    flushAnimationFrame?.(0)

    expect(store.messages).toMatchObject([
      {
        id: 'msg-1',
        conversationId: 'conv-1',
        role: 'assistant',
        content: 'Hello',
        thinking: 'Planning',
      },
    ])
  })

  it('flushes queued deltas before applying terminal stream events', async () => {
    const store = useAppStore()
    await store.selectConversation('conv-1')
    const socket = MockWebSocket.instances[0]
    if (!socket) {
      throw new Error('expected stream websocket to be created')
    }

    emit(socket, { type: 'token', messageId: 'msg-1', token: 'Done' })
    emit(socket, { type: 'done', messageId: 'msg-1', elapsedMs: 42 })

    expect(store.messages).toMatchObject([
      {
        id: 'msg-1',
        content: 'Done',
        elapsedMs: 42,
      },
    ])
  })

  it('uses terminal payload content and thinking when live deltas were missed', async () => {
    const store = useAppStore()
    await store.selectConversation('conv-1')
    const socket = MockWebSocket.instances[0]
    if (!socket) {
      throw new Error('expected stream websocket to be created')
    }

    emit(socket, {
      type: 'done',
      messageId: 'msg-1',
      content: 'Final answer',
      thinking: 'Final reasoning',
      elapsedMs: 84,
    })

    expect(store.messages).toMatchObject([
      {
        id: 'msg-1',
        conversationId: 'conv-1',
        role: 'assistant',
        content: 'Final answer',
        thinking: 'Final reasoning',
        elapsedMs: 84,
      },
    ])
  })

  it('reconciles terminal payloads with already streamed partial content', async () => {
    const store = useAppStore()
    await store.selectConversation('conv-1')
    const socket = MockWebSocket.instances[0]
    if (!socket) {
      throw new Error('expected stream websocket to be created')
    }

    emit(socket, { type: 'token', messageId: 'msg-1', token: 'Final' })
    emit(socket, { type: 'thinking', messageId: 'msg-1', thinking: 'Plan' })
    emit(socket, {
      type: 'done',
      messageId: 'msg-1',
      content: 'Final answer',
      thinking: 'Planning complete',
    })

    expect(store.messages).toMatchObject([
      {
        id: 'msg-1',
        content: 'Final answer',
        thinking: 'Planning complete',
      },
    ])
  })

  it('reopens the selected conversation stream after it was closed', async () => {
    const store = useAppStore()
    await store.selectConversation('conv-1')
    const socket = MockWebSocket.instances[0]
    if (!socket) {
      throw new Error('expected stream websocket to be created')
    }

    store.resumeSelectedConversationStream()

    expect(MockWebSocket.instances).toHaveLength(1)

    store.closeStream()
    emit(socket, { type: 'thinking', messageId: 'msg-1', thinking: 'stale' })
    flushAnimationFrame?.(0)

    expect(socket.close).toHaveBeenCalledOnce()
    expect(store.messages).toEqual([])

    store.resumeSelectedConversationStream()

    expect(MockWebSocket.instances).toHaveLength(2)
  })
})
