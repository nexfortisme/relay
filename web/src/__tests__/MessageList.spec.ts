import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ChatHeader from '../components/ChatHeader.vue'
import MessageList from '../components/MessageList.vue'
import type { DisplayMessage } from '../types'

type UrlObjectMethod = 'createObjectURL' | 'revokeObjectURL'

const originalCreateObjectURLDescriptor = Object.getOwnPropertyDescriptor(URL, 'createObjectURL')
const originalRevokeObjectURLDescriptor = Object.getOwnPropertyDescriptor(URL, 'revokeObjectURL')

function restoreUrlProperty(name: UrlObjectMethod, descriptor: PropertyDescriptor | undefined) {
  if (descriptor) {
    Object.defineProperty(URL, name, descriptor)
    return
  }
  delete (URL as unknown as Record<UrlObjectMethod, unknown>)[name]
}

function userMessage(
  attachments: { id: string; name: string }[],
): DisplayMessage {
  return {
    id: 'msg-1',
    conversationId: 'conv-1',
    role: 'user',
    content: 'Here are files',
    attachments,
    createdAt: '2026-04-25T12:00:00.000Z',
  }
}

function assistantMessage(content: string): DisplayMessage {
  return {
    id: 'msg-assistant-1',
    conversationId: 'conv-1',
    role: 'assistant',
    content,
    createdAt: '2026-04-25T12:00:01.000Z',
  }
}

describe('MessageList attachment preview', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    restoreUrlProperty('createObjectURL', originalCreateObjectURLDescriptor)
    restoreUrlProperty('revokeObjectURL', originalRevokeObjectURLDescriptor)
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('fetches and renders text-like attachments in the preview window', async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => new Response('{"b":2,"a":1}'))
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(MessageList, {
      attachTo: document.body,
      props: {
        messages: [userMessage([{ id: 'file-json', name: 'data.json' }])],
        pendingAssistant: false,
        theme: 'light',
      },
    })

    await wrapper.find('button[title="Preview data.json"]').trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith('http://localhost:8091/api/files/file-json/download')
    expect(document.body.textContent).toContain('"b": 2')
    expect(document.body.textContent).toContain('"a": 1')
    expect(document.body.querySelector('.file-preview-overlay')?.getAttribute('data-theme')).toBe(
      'light',
    )

    wrapper.unmount()
  })

  it('uses a blob URL for PDF previews and revokes it when closed', async () => {
    const fetchMock = vi.fn<typeof fetch>(async () => {
      return new Response(new Blob(['pdf'], { type: 'application/pdf' }))
    })
    const createObjectURL = vi.fn<() => string>(() => 'blob:pdf-preview')
    const revokeObjectURL = vi.fn<() => void>()
    vi.stubGlobal('fetch', fetchMock)
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURL,
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: revokeObjectURL,
    })

    const wrapper = mount(MessageList, {
      attachTo: document.body,
      props: {
        messages: [userMessage([{ id: 'file-pdf', name: 'paper.pdf' }])],
        pendingAssistant: false,
      },
    })

    await wrapper.find('button[title="Preview paper.pdf"]').trigger('click')
    await flushPromises()

    expect(createObjectURL).toHaveBeenCalledOnce()
    expect(document.body.querySelector('iframe')?.getAttribute('src')).toBe('blob:pdf-preview')

    document.body.querySelector<HTMLButtonElement>('button[aria-label="Close preview"]')?.click()
    await flushPromises()

    expect(revokeObjectURL).toHaveBeenCalledWith('blob:pdf-preview')

    wrapper.unmount()
  })

})

describe('MessageList code block copy', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('copies fenced code block content from assistant markdown', async () => {
    const writeText = vi.fn(async () => {})
    vi.stubGlobal('navigator', { clipboard: { writeText } })

    const wrapper = mount(MessageList, {
      attachTo: document.body,
      props: {
        messages: [assistantMessage('```ts\nconst value = 42;\n```')],
        pendingAssistant: false,
      },
    })

    await wrapper.find('.code-copy-button').trigger('click')

    expect(writeText).toHaveBeenCalledWith('const value = 42;')

    wrapper.unmount()
  })
})

describe('MessageList scrolling', () => {
  it('resets accidental horizontal drift when auto-scrolling', async () => {
    const wrapper = mount(MessageList, {
      attachTo: document.body,
      props: {
        messages: [userMessage([])],
        pendingAssistant: false,
      },
    })
    const messagesEl = wrapper.find('.messages').element as HTMLElement
    Object.defineProperty(messagesEl, 'scrollHeight', {
      configurable: true,
      value: 960,
    })

    messagesEl.scrollLeft = 320
    messagesEl.scrollTop = 12
    await (wrapper.vm as unknown as { scrollToBottom: () => Promise<void> }).scrollToBottom()

    expect(messagesEl.scrollLeft).toBe(0)
    expect(messagesEl.scrollTop).toBe(960)

    wrapper.unmount()
  })
})

describe('ChatHeader token meter', () => {
  it('renders conversation token usage with the configured cap', () => {
    const wrapper = mount(ChatHeader, {
      props: {
        isEditing: false,
        isRenaming: false,
        isSuggestingTitle: false,
        renameDraft: '',
        selectedConversationId: 'conv-1',
        title: 'Conversation',
        tokenCount: 42,
        maxTokenCount: 100,
      },
    })

    expect(wrapper.find('.conversation-token-meter').text()).toContain('42 / 100 tokens')

    wrapper.unmount()
  })
})
