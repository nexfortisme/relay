import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import {
  archiveConversation,
  deleteConversation,
  listConversations,
  renameConversation,
  restoreConversation,
  suggestConversationTitle,
  type Conversation,
} from '../lib/api'

export const DEFAULT_CONVERSATION_TITLE = 'New chat'

const MAX_CONVERSATION_TITLE_LENGTH = 40

function clampTitleForDisplay(title: string): string {
  const normalized = title.trim().replace(/\s+/g, ' ')
  if (normalized.length <= MAX_CONVERSATION_TITLE_LENGTH) {
    return normalized
  }
  return normalized.slice(0, MAX_CONVERSATION_TITLE_LENGTH).trim()
}

export const useConversationStore = defineStore('conversation', () => {
  const conversations = ref<Conversation[]>([])
  const selectedConversationId = ref<string | null>(null)
  const pendingRouteConversationId = ref<string | null>(null)
  const showArchived = ref(false)
  const renameDraft = ref('')
  const isRenaming = ref(false)
  const isSuggestingTitle = ref(false)
  const isEditingTitle = ref(false)

  const selectedConversation = computed(() =>
    conversations.value.find((c) => c.id === selectedConversationId.value),
  )

  const activeConversations = computed(() =>
    conversations.value.filter((c) => !c.archived),
  )

  async function loadConversations() {
    conversations.value = await listConversations(true)
  }

  function toggleArchived() {
    showArchived.value = !showArchived.value
  }

  function updateConversationInUrl(conversationId: string | null) {
    const url = new URL(window.location.href)
    if (conversationId) {
      url.searchParams.set('conversation', conversationId)
    } else {
      url.searchParams.delete('conversation')
    }
    const nextUrl = `${url.pathname}${url.search}${url.hash}`
    window.history.replaceState(window.history.state, '', nextUrl)
  }

  function getConversationIdFromUrl(): string | null {
    const url = new URL(window.location.href)
    return url.searchParams.get('conversation')
  }

  function beginConversationTitleEdit() {
    renameDraft.value = selectedConversation.value?.title ?? DEFAULT_CONVERSATION_TITLE
    isEditingTitle.value = true
  }

  function cancelConversationTitleEdit() {
    isEditingTitle.value = false
    renameDraft.value = selectedConversation.value?.title ?? DEFAULT_CONVERSATION_TITLE
  }

  async function saveConversationTitle() {
    if (!selectedConversationId.value) {
      return
    }
    const title = clampTitleForDisplay(renameDraft.value)
    if (!title) {
      renameDraft.value = selectedConversation.value?.title ?? DEFAULT_CONVERSATION_TITLE
      isEditingTitle.value = false
      return
    }
    isRenaming.value = true
    try {
      renameDraft.value = title
      await renameConversation(selectedConversationId.value, title)
      await loadConversations()
      isEditingTitle.value = false
    } finally {
      isRenaming.value = false
    }
  }

  async function suggestConversationTitleWithLLM() {
    if (!selectedConversationId.value || isRenaming.value || isSuggestingTitle.value) {
      return
    }
    isSuggestingTitle.value = true
    try {
      const suggestedTitle = await suggestConversationTitle(selectedConversationId.value)
      renameDraft.value = clampTitleForDisplay(suggestedTitle)
      await saveConversationTitle()
    } catch (error) {
      // Expose to callers via a re-throw; chat store will catch and display
      throw error
    } finally {
      isSuggestingTitle.value = false
    }
  }

  function confirmArchive(conversationId: string): boolean {
    const conversation = conversations.value.find((c) => c.id === conversationId)
    const title = conversation?.title ?? 'this chat'
    return window.confirm(`Archive "${title}"?`)
  }

  function confirmDelete(conversationId: string): boolean {
    const conversation = conversations.value.find((c) => c.id === conversationId)
    const title = conversation?.title ?? 'this chat'
    return window.confirm(`Delete "${title}"? This cannot be undone.`)
  }

  async function archiveConversationById(conversationId: string) {
    await archiveConversation(conversationId)
    await loadConversations()
  }

  async function restoreConversationById(conversationId: string) {
    await restoreConversation(conversationId)
    await loadConversations()
  }

  async function deleteConversationById(conversationId: string) {
    await deleteConversation(conversationId)
    await loadConversations()
  }

  return {
    conversations,
    selectedConversationId,
    pendingRouteConversationId,
    showArchived,
    renameDraft,
    isRenaming,
    isSuggestingTitle,
    isEditingTitle,
    selectedConversation,
    activeConversations,
    loadConversations,
    toggleArchived,
    updateConversationInUrl,
    getConversationIdFromUrl,
    beginConversationTitleEdit,
    cancelConversationTitleEdit,
    saveConversationTitle,
    suggestConversationTitleWithLLM,
    confirmArchive,
    confirmDelete,
    archiveConversationById,
    restoreConversationById,
    deleteConversationById,
  }
})
