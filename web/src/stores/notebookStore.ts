import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
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

export const useNotebookStore = defineStore('notebook', () => {
  const notebooks = ref<Notebook[]>([])
  const selectedNotebookId = ref<string | null>(null)
  const files = ref<NotebookFile[]>([])
  const conversations = ref<NotebookConversation[]>([])
  const selectedConversationId = ref<string | null>(null)
  const isLoading = ref(false)
  const isUploading = ref(false)
  const uploadProgress = ref(0)
  const error = ref<string | null>(null)

  let pollTimer: ReturnType<typeof setTimeout> | null = null

  const selectedNotebook = computed(() =>
    notebooks.value.find((n) => n.id === selectedNotebookId.value) ?? null,
  )

  const selectedConversation = computed(() =>
    conversations.value.find((c) => c.id === selectedConversationId.value) ?? null,
  )

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
      conversations.value = await listNotebookConversations(notebookId)
    } catch (e) {
      error.value = (e as Error).message
    }
  }

  function selectConversation(id: string | null) {
    selectedConversationId.value = id
  }

  function startPollIfNeeded(notebookId: string) {
    stopPolling()
    const hasPending = files.value.some(
      (f) => f.status === 'pending' || f.status === 'processing',
    )
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
  }): Promise<Notebook> {
    const nb = await createNotebook(payload)
    notebooks.value = [nb, ...notebooks.value]
    return nb
  }

  async function updateExistingNotebook(
    id: string,
    patch: Partial<{ name: string; description: string; systemPrompt: string; skillPrompt: string }>,
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
    return conv
  }

  return {
    notebooks,
    selectedNotebookId,
    selectedNotebook,
    files,
    conversations,
    selectedConversationId,
    selectedConversation,
    isLoading,
    isUploading,
    uploadProgress,
    error,
    loadNotebooks,
    selectNotebook,
    loadFiles,
    loadConversations,
    selectConversation,
    createNewNotebook,
    updateExistingNotebook,
    removeNotebook,
    uploadFile,
    removeFile,
    getCSVData,
    newChat,
    stopPolling,
  }
})
