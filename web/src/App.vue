<script setup lang="ts">
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import {
  archiveConversation,
  conversationHttpStreamUrl,
  conversationStreamUrl,
  createConversation,
  deleteConversation,
  createMessage,
  listConversations,
  listMessages,
  messageAttachmentDownloadUrl,
  renameConversation,
  restoreConversation,
  stopConversationGeneration,
  type Conversation,
  type Message,
} from './lib/api'

const conversations = ref<Conversation[]>([])
const selectedConversationId = ref<string | null>(null)
type DisplayMessage = Message & { thinking?: string }

const messages = ref<DisplayMessage[]>([])
const draft = ref('')
const selectedFiles = ref<File[]>([])
const isSending = ref(false)
const streamError = ref('')
const showArchived = ref(false)
const theme = ref<'dark' | 'light'>('dark')
const renameDraft = ref('')
const isRenaming = ref(false)
const isEditingTitle = ref(false)
const titleInputEl = ref<HTMLInputElement | null>(null)
const fileInputEl = ref<HTMLInputElement | null>(null)
const messagesEl = ref<HTMLElement | null>(null)
let streamSocket: WebSocket | null = null
let eventSourceFallback: EventSource | null = null
const maxTotalUploadBytes = parsePositiveInt(import.meta.env.VITE_MAX_UPLOAD_BYTES, 30 * 1024 * 1024)
const maxTotalUploadLabel = formatBytesLabel(maxTotalUploadBytes)
const maxImageUploadBytes = parsePositiveInt(import.meta.env.VITE_MAX_IMAGE_BYTES, 700 * 1024)
const maxImageUploadLabel = formatBytesLabel(maxImageUploadBytes)

const selectedConversation = computed(() =>
  conversations.value.find((c) => c.id === selectedConversationId.value),
)
const activeConversations = computed(() => conversations.value.filter((c) => !c.archived))
const archivedConversations = computed(() => conversations.value.filter((c) => c.archived))

const displayMessages = computed(() => messages.value)

function parsePositiveInt(value: string | undefined, fallback: number): number {
  if (!value) {
    return fallback
  }
  const parsed = Number.parseInt(value, 10)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return fallback
  }
  return parsed
}

function formatBytesLabel(bytes: number): string {
  if (bytes >= 1024 * 1024) {
    const mb = bytes / (1024 * 1024)
    return Number.isInteger(mb) ? `${mb}MB` : `${mb.toFixed(1)}MB`
  }
  if (bytes >= 1024) {
    const kb = bytes / 1024
    return Number.isInteger(kb) ? `${kb}KB` : `${kb.toFixed(1)}KB`
  }
  return `${bytes}B`
}

function displayUserMessage(message: DisplayMessage): string {
  const fromUserContent = message.userContent?.trim()
  if (fromUserContent) {
    return fromUserContent
  }
  const legacy = message.content
  const divider = '\n\n---\n'
  const dividerIndex = legacy.indexOf(divider)
  if (dividerIndex >= 0) {
    return legacy.slice(0, dividerIndex).trim()
  }
  return legacy
}

marked.setOptions({
  gfm: true,
  breaks: true,
})

onMounted(async () => {
  await loadConversations()
  const requestedConversationId = getConversationIdFromUrl()
  if (requestedConversationId) {
    const requestedConversation = conversations.value.find((conversation) => conversation.id === requestedConversationId)
    if (requestedConversation) {
      await selectConversation(requestedConversation.id)
      return
    }
  }
  if (activeConversations.value.length === 0) {
    await handleCreateConversation()
    return
  }
  const firstConversation = activeConversations.value[0]
  if (!firstConversation) {
    return
  }
  await selectConversation(firstConversation.id)
})

onUnmounted(() => {
  if (streamSocket) {
    streamSocket.close()
  }
  if (eventSourceFallback) {
    eventSourceFallback.close()
  }
})

async function loadConversations() {
  conversations.value = await listConversations(true)
}

async function handleCreateConversation() {
  const conversation = await createConversation()
  await loadConversations()
  await selectConversation(conversation.id)
}

async function selectConversation(conversationId: string) {
  selectedConversationId.value = conversationId
  updateConversationInUrl(conversationId)
  messages.value = await listMessages(conversationId)
  renameDraft.value = selectedConversation.value?.title ?? ''
  isEditingTitle.value = false
  setupStream(conversationId)
  await scrollMessagesToBottom()
}

function getConversationIdFromUrl(): string | null {
  const url = new URL(window.location.href)
  return url.searchParams.get('conversation')
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

type StreamPayload = {
  type: string
  messageId?: string
  token?: string
  thinking?: string
  error?: string
}

function setupStream(conversationId: string) {
  if (streamSocket) {
    streamSocket.close()
  }
  if (eventSourceFallback) {
    eventSourceFallback.close()
    eventSourceFallback = null
  }
  streamError.value = ''
  let hasOpenedWebSocket = false
  streamSocket = new WebSocket(conversationStreamUrl(conversationId))

  streamSocket.onopen = () => {
    hasOpenedWebSocket = true
  }

  streamSocket.onmessage = (event) => {
    const payload = JSON.parse(event.data) as StreamPayload
    applyStreamPayload(conversationId, payload)
  }

  streamSocket.onerror = () => {
    if (!hasOpenedWebSocket) {
      setupEventSourceFallback(conversationId)
      return
    }
    streamError.value = 'Stream disconnected'
    isSending.value = false
  }

  streamSocket.onclose = (event) => {
    if (!hasOpenedWebSocket) {
      setupEventSourceFallback(conversationId)
      return
    }
    if (!event.wasClean) {
      streamError.value = `Stream closed (code ${event.code})`
      isSending.value = false
    }
  }
}

function applyStreamPayload(conversationId: string, payload: StreamPayload) {
  if (payload.type === 'token' && payload.messageId) {
    upsertAssistantMessage(conversationId, payload.messageId, payload.token ?? '')
    void scrollMessagesToBottom()
    return
  }

  if (payload.type === 'thinking' && payload.messageId) {
    upsertAssistantThinking(conversationId, payload.messageId, payload.thinking ?? '')
    return
  }

  if (payload.type === 'done') {
    isSending.value = false
    return
  }

  if (payload.type === 'stopped') {
    isSending.value = false
    return
  }

  if (payload.type === 'error') {
    streamError.value = payload.error ?? 'Stream error'
    isSending.value = false
  }
}

function setupEventSourceFallback(conversationId: string) {
  if (eventSourceFallback) {
    eventSourceFallback.close()
  }
  eventSourceFallback = new EventSource(conversationHttpStreamUrl(conversationId))
  streamError.value = 'WebSocket failed, using fallback stream'

  eventSourceFallback.addEventListener('token', (event) => {
    const payload = JSON.parse((event as MessageEvent).data) as StreamPayload
    applyStreamPayload(conversationId, payload)
  })
  eventSourceFallback.addEventListener('thinking', (event) => {
    const payload = JSON.parse((event as MessageEvent).data) as StreamPayload
    applyStreamPayload(conversationId, payload)
  })
  eventSourceFallback.addEventListener('done', (event) => {
    const payload = JSON.parse((event as MessageEvent).data) as StreamPayload
    applyStreamPayload(conversationId, payload)
  })
  eventSourceFallback.addEventListener('error', (event) => {
    const messageEvent = event as MessageEvent
    const payload = safeParseStreamPayload(messageEvent.data)
    applyStreamPayload(conversationId, payload)
    streamError.value = payload.error ?? 'Stream disconnected'
    isSending.value = false
  })
}

function safeParseStreamPayload(raw: unknown): StreamPayload {
  if (typeof raw !== 'string') {
    return { type: 'error', error: 'Stream disconnected' }
  }
  try {
    return JSON.parse(raw) as StreamPayload
  } catch {
    return { type: 'error', error: 'Stream disconnected' }
  }
}

function upsertAssistantMessage(conversationId: string, messageId: string, token: string) {
  const existing = messages.value.find((message) => message.id === messageId)
  if (existing) {
    existing.content += token
    return
  }
  messages.value.push({
    id: messageId,
    conversationId,
    role: 'assistant',
    content: token,
    createdAt: new Date().toISOString(),
  })
}

function upsertAssistantThinking(conversationId: string, messageId: string, thinking: string) {
  if (!thinking) {
    return
  }
  const existing = messages.value.find((message) => message.id === messageId)
  if (existing) {
    existing.thinking = (existing.thinking ?? '') + thinking
    return
  }
  messages.value.push({
    id: messageId,
    conversationId,
    role: 'assistant',
    content: '',
    thinking,
    createdAt: new Date().toISOString(),
  })
}

async function beginConversationTitleEdit() {
  renameDraft.value = selectedConversation.value?.title ?? 'New chat'
  isEditingTitle.value = true
  await nextTick()
  titleInputEl.value?.focus()
  titleInputEl.value?.select()
}

function cancelConversationTitleEdit() {
  isEditingTitle.value = false
  renameDraft.value = selectedConversation.value?.title ?? 'New chat'
}

async function sendMessage() {
  const content = draft.value.trim()
  if (!content || !selectedConversationId.value || isSending.value) {
    return
  }

  const conversationId = selectedConversationId.value
  const filesToSend = [...selectedFiles.value]
  messages.value.push({
    id: `local-${Date.now()}`,
    conversationId,
    role: 'user',
    content,
    userContent: content,
    llmContent: content,
    attachments: filesToSend.map((file) => file.name),
    createdAt: new Date().toISOString(),
  })

  draft.value = ''
  selectedFiles.value = []
  if (fileInputEl.value) {
    fileInputEl.value.value = ''
  }
  isSending.value = true
  try {
    await createMessage(conversationId, content, filesToSend)
    await loadConversations()
    await scrollMessagesToBottom()
  } catch (error) {
    isSending.value = false
    streamError.value = error instanceof Error ? error.message : 'Failed to send message'
  }
}

function handleFileSelection(event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files ? Array.from(input.files) : []
  const totalBytes = files.reduce((sum, file) => sum + file.size, 0)
  if (totalBytes > maxTotalUploadBytes) {
    selectedFiles.value = []
    input.value = ''
    streamError.value = `Selected files exceed the ${maxTotalUploadLabel} total upload limit. Remove some files and try again.`
    return
  }
  const oversizedImages = files.filter(
    (file) => file.type.startsWith('image/') && file.size > maxImageUploadBytes,
  )
  if (oversizedImages.length > 0) {
    selectedFiles.value = []
    input.value = ''
    streamError.value = `Image files must be ${maxImageUploadLabel} or smaller: ${oversizedImages.map((file) => file.name).join(', ')}`
    return
  }
  selectedFiles.value = files
  if (streamError.value.includes('upload limit') || streamError.value.includes('Image files must be')) {
    streamError.value = ''
  }
}

function removeSelectedFile(index: number) {
  selectedFiles.value.splice(index, 1)
}

function openFilePicker() {
  fileInputEl.value?.click()
}

function toggleTheme() {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
}

async function saveConversationTitle() {
  if (!selectedConversationId.value) {
    return
  }
  const title = renameDraft.value.trim()
  if (!title) {
    renameDraft.value = selectedConversation.value?.title ?? 'New chat'
    isEditingTitle.value = false
    return
  }
  isRenaming.value = true
  try {
    await renameConversation(selectedConversationId.value, title)
    await loadConversations()
    isEditingTitle.value = false
  } finally {
    isRenaming.value = false
  }
}

async function stopGeneration() {
  if (!selectedConversationId.value || !isSending.value) {
    return
  }
  await stopConversationGeneration(selectedConversationId.value)
}

function confirmArchive(conversationId: string): boolean {
  const conversation = conversations.value.find((item) => item.id === conversationId)
  const title = conversation?.title ?? 'this chat'
  return window.confirm(`Archive "${title}"?`)
}

function confirmDelete(conversationId: string): boolean {
  const conversation = conversations.value.find((item) => item.id === conversationId)
  const title = conversation?.title ?? 'this chat'
  return window.confirm(`Delete "${title}"? This cannot be undone.`)
}

async function archiveSelectedConversation() {
  if (!selectedConversationId.value) {
    return
  }
  const toArchive = selectedConversationId.value
  if (!confirmArchive(toArchive)) {
    return
  }
  await archiveConversation(toArchive)
  await loadConversations()
  if (selectedConversationId.value === toArchive) {
    const replacement = activeConversations.value[0]
    if (replacement) {
      await selectConversation(replacement.id)
    } else {
      messages.value = []
      selectedConversationId.value = null
      updateConversationInUrl(null)
    }
  }
}

async function archiveChat(conversationId: string) {
  if (!confirmArchive(conversationId)) {
    return
  }
  await archiveConversation(conversationId)
  await loadConversations()
  if (selectedConversationId.value === conversationId) {
    const replacement = activeConversations.value[0]
    if (replacement) {
      await selectConversation(replacement.id)
    } else {
      messages.value = []
      selectedConversationId.value = null
      updateConversationInUrl(null)
    }
  }
}

async function restoreChat(conversationId: string) {
  await restoreConversation(conversationId)
  await loadConversations()
}

async function deleteChat(conversationId: string) {
  if (!confirmDelete(conversationId)) {
    return
  }
  await deleteConversation(conversationId)
  await loadConversations()
  if (selectedConversationId.value === conversationId) {
    const replacement = activeConversations.value[0]
    if (replacement) {
      await selectConversation(replacement.id)
    } else {
      messages.value = []
      selectedConversationId.value = null
      updateConversationInUrl(null)
    }
  }
}

async function scrollMessagesToBottom() {
  await nextTick()
  const el = messagesEl.value
  if (!el) {
    return
  }
  el.scrollTop = el.scrollHeight
}

function renderAssistantMarkdown(content: string): string {
  const parsed = marked.parse(content, { async: false })
  return DOMPurify.sanitize(parsed)
}

function renderThinkingMarkdown(content: string): string {
  const parsed = marked.parse(content, { async: false })
  return DOMPurify.sanitize(parsed)
}

function attachmentDownloadUrl(message: Message, attachmentIndex: number): string {
  return messageAttachmentDownloadUrl(message.conversationId, message.id, attachmentIndex)
}
</script>

<template>
  <main class="layout" :data-theme="theme">
    <aside class="sidebar">
      <div class="sidebar-actions">
        <button class="new-chat" @click="handleCreateConversation">New Chat</button>
        <button class="theme-toggle" @click="showArchived = !showArchived">
          {{ showArchived ? 'Hide archived' : 'Show archived' }}
        </button>
        <button class="theme-toggle" @click="toggleTheme">
          {{ theme === 'dark' ? 'Light mode' : 'Dark mode' }}
        </button>
      </div>
      <div class="conversation-list">
        <div
          v-for="conversation in activeConversations"
          :key="conversation.id"
          class="conversation-row"
          :class="{ active: conversation.id === selectedConversationId }"
        >
          <button class="conversation-item" @click="selectConversation(conversation.id)">
            {{ conversation.title }}
          </button>
          <button class="icon-button" title="Archive chat" @click.stop="archiveChat(conversation.id)">
            📦
          </button>
        </div>
        <div v-if="showArchived" class="archived-section">
          <div class="archived-title">Archived chats</div>
          <div
            v-for="conversation in archivedConversations"
            :key="conversation.id"
            class="conversation-row archived"
          >
            <button class="conversation-item" @click="selectConversation(conversation.id)">
              {{ conversation.title }}
            </button>
            <button class="icon-button" title="Restore chat" @click.stop="restoreChat(conversation.id)">
              ↩
            </button>
            <button class="icon-button" title="Delete chat" @click.stop="deleteChat(conversation.id)">
              🗑
            </button>
          </div>
        </div>
      </div>
    </aside>

    <section class="chat-panel">
      <header class="chat-header">
        <div class="chat-title-wrap">
          <template v-if="!isEditingTitle">
            <h1 class="chat-title">{{ selectedConversation?.title ?? 'New chat' }}</h1>
            <button class="title-edit-button" :disabled="isRenaming" @click="beginConversationTitleEdit">
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <path
                  d="M3 17.25V21h3.75L17.8 9.95l-3.75-3.75L3 17.25zm17.7-10.04a1 1 0 0 0 0-1.42l-2.48-2.48a1 1 0 0 0-1.42 0l-1.95 1.95 3.75 3.75 2.1-2.09z"
                />
              </svg>
              Rename
            </button>
            <button class="title-edit-button" :disabled="!selectedConversationId" @click="archiveSelectedConversation">
              Archive
            </button>
          </template>
          <template v-else>
            <input
              ref="titleInputEl"
              v-model="renameDraft"
              class="chat-title-input"
              :disabled="isRenaming"
              @blur="saveConversationTitle"
              @keydown.enter.prevent="saveConversationTitle"
              @keydown.esc.prevent="cancelConversationTitleEdit"
            />
          </template>
        </div>
      </header>
      <div ref="messagesEl" class="messages">
        <article
          v-for="message in displayMessages"
          :key="message.id"
          class="message"
          :class="message.role"
        >
          <details v-if="message.role === 'assistant' && message.thinking" class="message-thinking">
            <summary>Thinking</summary>
            <div class="message-markdown" v-html="renderThinkingMarkdown(message.thinking)" />
          </details>
          <strong class="message-role">{{ message.role }}</strong>
          <template v-if="message.role !== 'assistant'">
            <p>{{ displayUserMessage(message) }}</p>
            <div v-if="message.attachments?.length" class="message-attachments">
              <a
                v-for="(attachment, index) in message.attachments"
                :key="`${attachment}-${index}`"
                class="message-attachment-chip"
                :href="attachmentDownloadUrl(message, index)"
                :download="attachment"
                :title="`Download ${attachment}`"
              >
                <span aria-hidden="true">📄</span>
                {{ attachment }}
              </a>
            </div>
          </template>
          <div
            v-else
            class="message-markdown"
            v-html="renderAssistantMarkdown(message.content)"
          />
        </article>
      </div>
      <p v-if="streamError" class="error">{{ streamError }}</p>
      <form class="composer" @submit.prevent="sendMessage">
        <div v-if="selectedFiles.length" class="file-list">
          <span v-for="(file, index) in selectedFiles" :key="`${file.name}-${index}`" class="file-chip">
            {{ file.name }}
            <button type="button" class="file-chip-remove" @click="removeSelectedFile(index)">x</button>
          </span>
        </div>
        <button
          type="button"
          class="file-picker-button"
          aria-label="Upload files"
          title="Upload files"
          @click="openFilePicker"
        >
          +
        </button>
        <input
          ref="fileInputEl"
          class="file-picker-hidden"
          type="file"
          multiple
          accept="image/*,.pdf,.txt,.md,.markdown,.json,.csv,.xml,.yaml,.yml"
          @change="handleFileSelection"
        />
        <input v-model="draft" placeholder="Ask something..." />
        <button
          :type="isSending ? 'button' : 'submit'"
          :disabled="!isSending && !draft.trim()"
          class="composer-send-button"
          :class="{ 'stop-button': isSending }"
          @click="isSending ? stopGeneration() : undefined"
        >
          {{ isSending ? 'Stop' : 'Send' }}
        </button>
      </form>
    </section>
  </main>
</template>

<style scoped>
:global(html, body, #app) {
  margin: 0;
  height: 100%;
  overflow: hidden;
}

.layout {
  display: grid;
  grid-template-columns: 300px 1fr;
  height: 100dvh;
  overflow: hidden;
  background: var(--bg);
  color: var(--text);
}

.layout[data-theme='dark'] {
  --bg: #0b1220;
  --surface: #121a2b;
  --surface-soft: #1a2438;
  --text: #e6edf8;
  --muted: #8ea0bf;
  --border: #27324a;
  --primary: #4f7cff;
  --primary-strong: #3e66e5;
}

.layout[data-theme='light'] {
  --bg: #f3f6fc;
  --surface: #ffffff;
  --surface-soft: #f7f9ff;
  --text: #0f172a;
  --muted: #475569;
  --border: #d8e0f0;
  --primary: #3368ff;
  --primary-strong: #2553d8;
}

.sidebar {
  border-right: 1px solid var(--border);
  padding: 1rem;
  background: var(--surface);
  display: grid;
  grid-template-rows: auto 1fr;
  gap: 1rem;
  overflow: hidden;
}

.sidebar-actions {
  display: grid;
  gap: 0.6rem;
}

.new-chat {
  width: 100%;
  padding: 0.65rem 0.75rem;
  border-radius: 0.6rem;
  border: none;
  background: var(--primary);
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.theme-toggle {
  width: 100%;
  padding: 0.6rem 0.75rem;
  border-radius: 0.6rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  font-weight: 600;
  cursor: pointer;
}

.conversation-list {
  display: grid;
  gap: 0.32rem;
  overflow: auto;
  align-content: start;
}

.conversation-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 0.3rem;
}

.conversation-row.archived {
  grid-template-columns: 1fr auto auto;
}

.conversation-row.active .conversation-item {
  border-color: var(--primary);
  background: color-mix(in srgb, var(--primary) 12%, var(--surface));
}

.archived-section {
  margin-top: 0.6rem;
  display: grid;
  gap: 0.3rem;
}

.archived-title {
  font-size: 0.75rem;
  color: var(--muted);
  text-transform: uppercase;
}

.conversation-item {
  text-align: left;
  padding: 0.38rem 0.5rem;
  border-radius: 0.45rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  cursor: pointer;
  font-size: 0.84rem;
  line-height: 1.2;
  width: 100%;
  box-sizing: border-box;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.icon-button {
  border: 1px solid var(--border);
  border-radius: 0.45rem;
  background: var(--surface-soft);
  color: var(--text);
  cursor: pointer;
  padding: 0.1rem 0.35rem;
}

.chat-panel {
  display: grid;
  grid-template-rows: auto 1fr auto;
  overflow: hidden;
  background: var(--bg);
  position: relative;
}

.chat-header {
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border);
}

.chat-title-wrap {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
}

.chat-title {
  margin: 0;
  font-size: 1.8rem;
  font-weight: 700;
}

.chat-title-input {
  font-size: 1.5rem;
  font-weight: 700;
  border: none;
  background: transparent;
  color: var(--text);
  outline: none;
  min-width: 220px;
  width: min(640px, 100%);
}

.title-edit-button {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  border-radius: 0.6rem;
  padding: 0.4rem 0.65rem;
  font-size: 0.85rem;
  cursor: pointer;
}

.title-edit-button svg {
  width: 0.95rem;
  height: 0.95rem;
  fill: currentColor;
}

.messages {
  padding: 1rem 1.25rem 7rem;
  overflow: auto;
  display: grid;
  gap: 0.65rem;
  align-content: start;
}

.message {
  padding: 0.45rem 0.62rem;
  border-radius: 0.85rem;
  max-width: min(60%, 560px);
  line-height: 1.5;
  width: fit-content;
}

.message.user {
  margin-left: auto;
  background: var(--primary);
  color: #fff;
}

.message.assistant {
  margin-right: auto;
  background: var(--surface);
  border: 1px solid var(--border);
}

.message-role {
  display: block;
  font-size: 0.67rem;
  text-transform: uppercase;
  opacity: 0.75;
  margin-bottom: 0.2rem;
}

.message p {
  margin: 0;
  font-size: 0.93rem;
}

.message-attachments {
  margin-top: 0.4rem;
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.message-attachment-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  padding: 0.2rem 0.45rem;
  font-size: 0.75rem;
  color: inherit;
  text-decoration: none;
}

.message :deep(.message-markdown) {
  font-size: 0.93rem;
}

.message :deep(.message-markdown > :first-child) {
  margin-top: 0;
}

.message :deep(.message-markdown > :last-child) {
  margin-bottom: 0;
}

.message :deep(.message-markdown pre) {
  overflow-x: auto;
  padding: 0.6rem;
  border-radius: 0.5rem;
  background: var(--surface-soft);
}

.message :deep(.message-markdown code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 0.88em;
}

.message :deep(.message-markdown p),
.message :deep(.message-markdown ul),
.message :deep(.message-markdown ol),
.message :deep(.message-markdown blockquote) {
  margin: 0.4rem 0;
}

.message :deep(.message-markdown a) {
  color: var(--primary);
}

.message-thinking {
  margin: 0 0 0.35rem;
  border: 1px solid var(--border);
  border-radius: 0.55rem;
  background: var(--surface-soft);
  padding: 0.35rem 0.5rem;
  font-size: 0.8rem;
}

.message-thinking summary {
  cursor: pointer;
  font-weight: 600;
}

.message-thinking p {
  margin-top: 0.35rem;
  white-space: pre-wrap;
}

.composer {
  position: absolute;
  left: 1.25rem;
  right: 1.25rem;
  bottom: 0.9rem;
  padding: 0.65rem;
  display: grid;
  grid-template-columns: auto 1fr auto;
  grid-template-rows: auto auto;
  grid-template-areas:
    'files files files'
    'upload input send';
  gap: 0.6rem;
  border: 1px solid var(--border);
  border-radius: 0.85rem;
  background: var(--surface);
  box-shadow: 0 12px 28px rgba(2, 10, 30, 0.22);
}

.file-list {
  grid-area: files;
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}

.file-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  padding: 0.2rem 0.45rem;
  font-size: 0.75rem;
}

.file-chip-remove {
  border: none;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}

.file-picker-button {
  grid-area: upload;
  width: 2.6rem;
  min-width: 2.6rem;
  padding: 0.75rem 0;
}

.file-picker-hidden {
  display: none;
}

.stop-button {
  background: #c2410c;
}

.composer input {
  grid-area: input;
  padding: 0.75rem 0.85rem;
  border-radius: 0.6rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
}

.composer button {
  padding: 0.75rem 1rem;
  border-radius: 0.6rem;
  border: none;
  background: var(--primary);
  color: #fff;
  font-weight: 600;
  cursor: pointer;
}

.composer-send-button {
  grid-area: send;
}

.composer button:disabled {
  opacity: 0.6;
  background: var(--primary-strong);
}

.error {
  color: #ef4444;
  padding: 0 1.25rem 0.5rem;
}
</style>
