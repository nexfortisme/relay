import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import {
  archiveConversation,
  deleteConversation,
  renameConversation,
  restoreConversation,
  suggestConversationTitle,
  updateConversationFavorite,
} from '../lib/api'
import {
  createNotebook,
  createNotebookConversation,
  deleteNotebook,
  deleteNotebookFile,
  getCSVTableData,
  getPendingJobCount,
  listNotebookConversations,
  listNotebookFiles,
  listNotebooks,
  updateNotebook,
  uploadNotebookFile,
  type CSVTableData,
  type Notebook,
  type NotebookConversation,
  type NotebookFile,
} from '../lib/notebooks'

export const DEFAULT_NOTEBOOK_CONVERSATION_TITLE = 'New chat'

const MAX_CONVERSATION_TITLE_LENGTH = 40

function clampTitleForDisplay(title: string): string {
  const normalized = title.trim().replace(/\s+/g, ' ')
  if (normalized.length <= MAX_CONVERSATION_TITLE_LENGTH) {
    return normalized
  }
  return normalized.slice(0, MAX_CONVERSATION_TITLE_LENGTH).trim()
}

export const useNotebookStore = defineStore('notebook', () => {
  const notebooks = ref<Notebook[]>([])
  const selectedNotebookId = ref<string | null>(null)
  const files = ref<NotebookFile[]>([])
  const conversations = ref<NotebookConversation[]>([])
  const selectedConversationId = ref<string | null>(null)
  const showArchived = ref(false)
  const isLoading = ref(false)
  const isUploading = ref(false)
  const uploadProgress = ref(0)
  const error = ref<string | null>(null)
  const renameDraft = ref('')
  const isRenaming = ref(false)
  const isSuggestingTitle = ref(false)
  const isEditingTitle = ref(false)

  let pollTimer: ReturnType<typeof setTimeout> | null = null

  const selectedNotebook = computed(
    () => notebooks.value.find((n) => n.id === selectedNotebookId.value) ?? null,
  )

  const selectedConversation = computed(
    () => conversations.value.find((c) => c.id === selectedConversationId.value) ?? null,
  )

  const activeConversations = computed(() => conversations.value.filter((c) => !c.archived))

  const favoriteConversations = computed(() => activeConversations.value.filter((c) => c.favorite))

  const regularConversations = computed(() => activeConversations.value.filter((c) => !c.favorite))

  const archivedConversations = computed(() => conversations.value.filter((c) => c.archived))

  async function loadNotebooks() {
    isLoading.value = true
    error.value = null
    try {
      notebooks.value = await listNotebooks()
    } catch (e) {
      error.value = (e as Error).message
    } finally {
      isLoading.value = false
    }
  }

  async function selectNotebook(id: string | null) {
    selectedNotebookId.value = id
    selectedConversationId.value = null
    files.value = []
    conversations.value = []
    stopPolling()
    if (id) {
      await Promise.all([loadFiles(id), loadConversations(id)])
      startPollIfNeeded(id)
    }
  }

  async function loadFiles(notebookId: string) {
    try {
      files.value = await listNotebookFiles(notebookId)
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  async function loadConversations(notebookId: string) {
    try {
      conversations.value = await listNotebookConversations(notebookId, true)
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  function selectConversation(id: string | null) {
    selectedConversationId.value = id
    renameDraft.value = selectedConversation.value?.title ?? DEFAULT_NOTEBOOK_CONVERSATION_TITLE
    isEditingTitle.value = false
  }

  function toggleArchived() {
    showArchived.value = !showArchived.value
  }

  function beginConversationTitleEdit() {
    renameDraft.value = selectedConversation.value?.title ?? DEFAULT_NOTEBOOK_CONVERSATION_TITLE
    isEditingTitle.value = true
  }

  function cancelConversationTitleEdit() {
    isEditingTitle.value = false
    renameDraft.value = selectedConversation.value?.title ?? DEFAULT_NOTEBOOK_CONVERSATION_TITLE
  }

  async function saveConversationTitle() {
    if (!selectedNotebookId.value || !selectedConversationId.value) {
      return
    }
    const title = clampTitleForDisplay(renameDraft.value)
    if (!title) {
      renameDraft.value = selectedConversation.value?.title ?? DEFAULT_NOTEBOOK_CONVERSATION_TITLE
      isEditingTitle.value = false
      return
    }
    isRenaming.value = true
    try {
      renameDraft.value = title
      await renameConversation(selectedConversationId.value, title)
      await loadConversations(selectedNotebookId.value)
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
    } finally {
      isSuggestingTitle.value = false
    }
  }

  function startPollIfNeeded(notebookId: string) {
    stopPolling()
    const hasPending = files.value.some((f) => f.status === 'pending' || f.status === 'processing')
    if (!hasPending) return
    schedulePoll(notebookId)
  }

  function schedulePoll(notebookId: string) {
    pollTimer = setTimeout(async () => {
      try {
        const count = await getPendingJobCount(notebookId)
        if (selectedNotebookId.value === notebookId) {
          await loadFiles(notebookId)
          notebooks.value = await listNotebooks()
        }
        if (count > 0 && selectedNotebookId.value === notebookId) {
          schedulePoll(notebookId)
        } else {
          pollTimer = null
        }
      } catch {
        pollTimer = null
      }
    }, 5000)
  }

  function stopPolling() {
    if (pollTimer !== null) {
      clearTimeout(pollTimer)
      pollTimer = null
    }
  }

  async function createNewNotebook(payload: {
    name: string
    description?: string
    systemPrompt?: string
    skillPrompt?: string
    includeInGeneral?: boolean
  }): Promise<Notebook> {
    const nb = await createNotebook(payload)
    notebooks.value = [nb, ...notebooks.value]
    return nb
  }

  async function updateExistingNotebook(
    id: string,
    patch: Partial<{
      name: string
      description: string
      systemPrompt: string
      skillPrompt: string
      includeInGeneral: boolean
    }>,
  ): Promise<void> {
    const nb = await updateNotebook(id, patch)
    const idx = notebooks.value.findIndex((n) => n.id === id)
    if (idx !== -1) notebooks.value[idx] = nb
  }

  async function removeNotebook(id: string): Promise<void> {
    await deleteNotebook(id)
    notebooks.value = notebooks.value.filter((n) => n.id !== id)
    if (selectedNotebookId.value === id) {
      selectedNotebookId.value = null
      selectedConversationId.value = null
      files.value = []
      conversations.value = []
      stopPolling()
    }
  }

  async function uploadFile(
    notebookId: string,
    file: File,
    onProgress?: (pct: number) => void,
  ): Promise<NotebookFile> {
    isUploading.value = true
    uploadProgress.value = 0
    try {
      const uploaded = await uploadNotebookFile(notebookId, file, (pct) => {
        uploadProgress.value = pct
        onProgress?.(pct)
      })
      files.value = [...files.value, uploaded]
      notebooks.value = await listNotebooks()
      startPollIfNeeded(notebookId)
      return uploaded
    } finally {
      isUploading.value = false
      uploadProgress.value = 0
    }
  }

  async function removeFile(notebookId: string, fileId: string): Promise<void> {
    await deleteNotebookFile(notebookId, fileId)
    files.value = files.value.filter((f) => f.id !== fileId)
    notebooks.value = await listNotebooks()
  }

  async function getCSVData(notebookId: string, fileId: string): Promise<CSVTableData> {
    return getCSVTableData(notebookId, fileId)
  }

  async function newChat(notebookId: string): Promise<NotebookConversation> {
    const conv = await createNotebookConversation(notebookId)
    conversations.value = [conv, ...conversations.value]
    selectedConversationId.value = conv.id
    renameDraft.value = conv.title
    return conv
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
    if (!selectedNotebookId.value) return
    await archiveConversation(conversationId)
    await loadConversations(selectedNotebookId.value)
  }

  async function restoreConversationById(conversationId: string) {
    if (!selectedNotebookId.value) return
    await restoreConversation(conversationId)
    await loadConversations(selectedNotebookId.value)
  }

  async function deleteConversationById(conversationId: string) {
    if (!selectedNotebookId.value) return
    await deleteConversation(conversationId)
    conversations.value = conversations.value.filter((c) => c.id !== conversationId)
    await loadConversations(selectedNotebookId.value)
  }

  async function setConversationFavoriteById(conversationId: string, favorite: boolean) {
    if (!selectedNotebookId.value) return
    await updateConversationFavorite(conversationId, favorite)
    await loadConversations(selectedNotebookId.value)
  }

  async function toggleConversationFavoriteById(conversationId: string) {
    const conversation = conversations.value.find((c) => c.id === conversationId)
    await setConversationFavoriteById(conversationId, !(conversation?.favorite ?? false))
  }

  return {
    notebooks,
    selectedNotebookId,
    selectedNotebook,
    files,
    conversations,
    selectedConversationId,
    selectedConversation,
    activeConversations,
    favoriteConversations,
    regularConversations,
    archivedConversations,
    showArchived,
    isLoading,
    isUploading,
    uploadProgress,
    error,
    renameDraft,
    isRenaming,
    isSuggestingTitle,
    isEditingTitle,
    loadNotebooks,
    selectNotebook,
    loadFiles,
    loadConversations,
    selectConversation,
    toggleArchived,
    beginConversationTitleEdit,
    cancelConversationTitleEdit,
    saveConversationTitle,
    suggestConversationTitleWithLLM,
    createNewNotebook,
    updateExistingNotebook,
    removeNotebook,
    uploadFile,
    removeFile,
    getCSVData,
    newChat,
    confirmArchive,
    confirmDelete,
    archiveConversationById,
    restoreConversationById,
    deleteConversationById,
    setConversationFavoriteById,
    toggleConversationFavoriteById,
    stopPolling,
  }
})
