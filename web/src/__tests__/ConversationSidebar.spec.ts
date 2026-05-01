import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { describe, expect, it } from 'vitest'
import ConversationSidebar from '../components/ConversationSidebar.vue'
import type { Conversation } from '../lib/api'

function conversation(id: string, title: string, archived = false): Conversation {
  return {
    id,
    title,
    archived,
    createdAt: '2026-04-25T12:00:00.000Z',
    updatedAt: '2026-04-25T12:00:00.000Z',
  }
}

describe('ConversationSidebar', () => {
  it('renders the generating indicator inside the conversation text button', () => {
    const wrapper = mount(ConversationSidebar, {
      global: {
        plugins: [createPinia()],
        stubs: { UserMenu: true, RouterLink: true },
      },
      props: {
        conversations: [conversation('conv-1', 'Summarize this')],
        generatingConversationId: 'conv-1',
        selectedConversationId: 'conv-1',
        showArchived: false,
        theme: 'dark',
      },
    })

    const item = wrapper.find('.conversation-item')
    const indicator = wrapper.find('.sidebar-generating-indicator')

    expect(item.exists()).toBe(true)
    expect(indicator.exists()).toBe(true)
    expect(item.element.contains(indicator.element)).toBe(true)
  })
})
