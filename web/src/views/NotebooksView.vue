<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import type { NotebookFile } from '../lib/notebooks'
import { storeToRefs } from 'pinia'
import { useRouter } from 'vue-router'
import AppIcon from '../components/AppIcon.vue'
import ChatComposer from '../components/ChatComposer.vue'
import EmptyChatGreeting from '../components/EmptyChatGreeting.vue'
import MessageList from '../components/MessageList.vue'
import NotebookCreateDialog from '../components/NotebookCreateDialog.vue'
import NotebookSettingsDialog from '../components/NotebookSettingsDialog.vue'
import NotebookSidebar from '../components/NotebookSidebar.vue'
import PageNavTabs from '../components/PageNavTabs.vue'
import { useNotebookStore } from '../stores/notebookStore'
import { useUiStore } from '../stores/uiStore'
import { useChatStore } from '../stores/chatStore'

const router = useRouter()
const uiStore = useUiStore()
const chatStore = useChatStore()
const notebookStore = useNotebookStore()
const { theme } = storeToRefs(uiStore)
const {
  notebooks,
  selectedNotebook,
  selectedNotebookId,
  files,
  conversations,
  selectedConversationId,
  isLoading,
  isUploading,
  uploadProgress,
  error,
} = storeToRefs(notebookStore)

const {
  messages,
  draft,
  isSending,
  generatingConversationId,
  selectedFiles,
  isConversationTokenCapReached,
  shouldShowPendingAssistantPlaceholder,
  streamError,
} = storeToRefs(chatStore)

const showCreateDialog = ref(false)
const showSettingsDialog = ref(false)
// 'chats' | 'files' — what the middle pane shows
const middleMode = ref<'chats' | 'files'>('chats')
const openErrorFileId = ref<string | null>(null)
const openProgressFileId = ref<string | null>(null)
// Ticks every second so elapsed-time displays stay live
const now = ref(Date.now())
let elapsedTimer: ReturnType<typeof setInterval> | null = null

const isGenerating = computed(
  () =>
    !!selectedConversationId.value &&
    generatingConversationId.value === selectedConversationId.value,
)
const shouldShowEmptyGreeting = computed(
  () => messages.value.length === 0 && !shouldShowPendingAssistantPlaceholder.value,
)

onMounted(async () => {
  await notebookStore.loadNotebooks()
  startElapsedTimer()
})

onUnmounted(() => {
  notebookStore.stopPolling()
  stopElapsedTimer()
})

async function handleCreate(payload: {
  name: string
  description: string
  systemPrompt: string
  files: File[]
}) {
  showCreateDialog.value = false
  const nb = await notebookStore.createNewNotebook({
    name: payload.name,
    description: payload.description || undefined,
    systemPrompt: payload.systemPrompt || undefined,
  })
  await notebookStore.selectNotebook(nb.id)
  for (const file of payload.files) {
    await notebookStore.uploadFile(nb.id, file)
  }
}

async function handleSaveSettings(
  patch: Partial<{ name: string; description: string; systemPrompt: string; skillPrompt: string }>,
) {
  showSettingsDialog.value = false
  if (!selectedNotebookId.value) return
  await notebookStore.updateExistingNotebook(selectedNotebookId.value, patch)
}

async function handleSelectNotebook(id: string) {
  await notebookStore.selectNotebook(id)
  // Stay in current mode — let user decide what to look at
}

async function handleFileUpload(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files || !selectedNotebook.value) return
  const notebookId = selectedNotebook.value.id
  for (const file of Array.from(input.files)) {
    await notebookStore.uploadFile(notebookId, file)
  }
  input.value = ''
}

async function selectConversation(id: string) {
  notebookStore.selectConversation(id)
  await chatStore.selectConversation(id)
}

async function createChat() {
  if (!selectedNotebookId.value) return
  const conv = await notebookStore.newChat(selectedNotebookId.value)
  await chatStore.selectConversation(conv.id)
  middleMode.value = 'chats'
}

function toggleMiddleMode() {
  middleMode.value = middleMode.value === 'files' ? 'chats' : 'files'
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

function statusLabel(status: string): string {
  switch (status) {
    case 'pending':
      return 'Queued'
    case 'processing':
      return 'Processing'
    case 'ready':
      return 'Ready'
    case 'error':
      return 'Error'
    default:
      return status
  }
}

function formatDate(iso: string | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return ''
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' }).format(d)
}

function toggleErrorFlyout(fileId: string) {
  openErrorFileId.value = openErrorFileId.value === fileId ? null : fileId
}

function toggleProgressFlyout(fileId: string) {
  openProgressFileId.value = openProgressFileId.value === fileId ? null : fileId
  openErrorFileId.value = null
}

type ProcessingStage = { label: string; note?: string }

function getProcessingStages(file: NotebookFile): ProcessingStage[] {
  if (file.fileKind === 'csv') {
    return [{ label: 'Parsing table structure' }, { label: 'Building search index' }]
  }
  if (file.fileKind === 'image') {
    return [{ label: 'Storing metadata' }]
  }
  const isPdf =
    file.contentType === 'application/pdf' || file.name.toLowerCase().endsWith('.pdf')
  if (isPdf) {
    return [
      { label: 'Extracting text' },
      { label: 'Rendering pages to images' },
      { label: 'Generating AI descriptions', note: 'may take a moment for large PDFs' },
      { label: 'Building search index' },
    ]
  }
  return [{ label: 'Extracting text' }, { label: 'Building search index' }]
}

function formatElapsed(isoDate: string): string {
  const secs = Math.max(0, Math.floor((now.value - new Date(isoDate).getTime()) / 1000))
  if (secs < 60) return `${secs}s`
  const m = Math.floor(secs / 60)
  const s = secs % 60
  return `${m}m ${s}s`
}

function startElapsedTimer() {
  if (elapsedTimer !== null) return
  elapsedTimer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
}

function stopElapsedTimer() {
  if (elapsedTimer !== null) {
    clearInterval(elapsedTimer)
    elapsedTimer = null
  }
}
</script>

<template>
  <div class="nb-view">
    <NotebookSidebar
      :notebooks="notebooks"
      :selected-notebook-id="selectedNotebookId"
      :showing-files="middleMode === 'files'"
      :is-loading="isLoading"
      @create="showCreateDialog = true"
      @home="router.push('/home')"
      @select="handleSelectNotebook"
      @toggle-files="toggleMiddleMode"
    />

    <section class="nb-shell">
      <PageNavTabs />

      <div class="nb-reader">
        <!-- ── Middle pane: chats or files ── -->
        <section class="middle-pane">
          <!-- Files mode -->
          <template v-if="middleMode === 'files'">
            <div class="pane-header">
              <div>
                <h2 class="pane-title">Files</h2>
                <p v-if="selectedNotebook" class="pane-subtitle">{{ selectedNotebook.name }}</p>
              </div>
              <label v-if="selectedNotebook" class="upload-label" title="Upload file">
                <AppIcon name="plus" :size="14" />
                Upload
                <input type="file" multiple class="sr-only" @change="handleFileUpload" />
              </label>
            </div>

            <div class="pane-body">
              <div v-if="!selectedNotebook" class="pane-empty">Select a notebook to manage files.</div>
              <template v-else>
                <div v-if="isUploading" class="upload-progress">
                  <div class="upload-bar" :style="{ width: uploadProgress + '%' }" />
                  <span>{{ uploadProgress }}%</span>
                </div>
                <p v-if="files.length === 0 && !isUploading" class="pane-empty">
                  No files yet — upload one above.
                </p>
                <div v-for="file in files" :key="file.id" class="file-entry">
                  <div class="file-row">
                    <span class="file-name" :title="file.name">{{ file.name }}</span>
                    <span class="file-size">{{ formatBytes(file.sizeBytes) }}</span>
                    <button
                      v-if="file.status === 'pending' || file.status === 'processing'"
                      class="file-status file-status-btn"
                      :class="[
                        `file-status--${file.status}`,
                        { 'file-status--open': openProgressFileId === file.id },
                      ]"
                      :title="'View processing stages'"
                      @click="toggleProgressFlyout(file.id)"
                    >
                      {{ statusLabel(file.status) }}
                    </button>
                    <span
                      v-else
                      class="file-status"
                      :class="`file-status--${file.status}`"
                    >
                      {{ statusLabel(file.status) }}
                    </span>
                    <button
                      v-if="file.error"
                      class="file-err-btn"
                      :class="{ 'file-err-btn--open': openErrorFileId === file.id }"
                      @click="toggleErrorFlyout(file.id)"
                    >
                      !
                    </button>
                    <button
                      class="file-del"
                      title="Remove"
                      @click="notebookStore.removeFile(selectedNotebook!.id, file.id)"
                    >
                      <AppIcon name="x" :size="11" />
                    </button>
                  </div>
                  <div v-if="file.error && openErrorFileId === file.id" class="file-flyout">
                    <pre class="file-flyout-msg">{{ file.error }}</pre>
                  </div>
                  <div
                    v-if="
                      (file.status === 'pending' || file.status === 'processing') &&
                      openProgressFileId === file.id
                    "
                    class="file-flyout progress-flyout"
                  >
                    <div class="progress-status">
                      <span v-if="file.status === 'processing'" class="progress-spinner" />
                      <span class="progress-status-text">
                        {{
                          file.status === 'pending'
                            ? 'Waiting in queue'
                            : `Processing… ${formatElapsed(file.updatedAt)}`
                        }}
                      </span>
                      <span
                        v-if="file.status === 'processing' && file.pageCount > 0"
                        class="progress-page-count"
                      >
                        {{ file.pagesIndexed }} / {{ file.pageCount }}
                        {{ file.pageCount === 1 ? 'page' : 'pages' }}
                      </span>
                    </div>
                    <div
                      v-if="file.status === 'processing' && file.pageCount > 0"
                      class="progress-bar-track"
                    >
                      <div
                        class="progress-bar-fill"
                        :style="{
                          width: `${Math.round((file.pagesIndexed / file.pageCount) * 100)}%`,
                        }"
                      />
                    </div>
                    <ul class="stage-list">
                      <li
                        v-for="(stage, idx) in getProcessingStages(file)"
                        :key="idx"
                        class="stage-item"
                      >
                        <span class="stage-dot" />
                        <span class="stage-label">{{ stage.label }}</span>
                        <span v-if="stage.note" class="stage-note">— {{ stage.note }}</span>
                      </li>
                    </ul>
                  </div>
                </div>
                <p v-if="error" class="pane-empty pane-error">{{ error }}</p>
              </template>
            </div>
          </template>

          <!-- Chats mode -->
          <template v-else>
            <div class="pane-header">
              <div>
                <h2 class="pane-title">{{ selectedNotebook?.name ?? 'Chats' }}</h2>
                <p v-if="selectedNotebook?.description" class="pane-subtitle">
                  {{ selectedNotebook.description }}
                </p>
              </div>
              <div class="header-actions">
                <button
                  v-if="selectedNotebook"
                  class="icon-btn"
                  title="Notebook settings"
                  @click="showSettingsDialog = true"
                >
                  <AppIcon name="settings" :size="15" />
                </button>
                <button v-if="selectedNotebook" class="primary-btn" @click="createChat">
                  <AppIcon name="plus" :size="14" />
                  New chat
                </button>
              </div>
            </div>

            <div class="pane-body">
              <div v-if="!selectedNotebook" class="pane-empty">
                Select a notebook to see its chats.
              </div>
              <div v-else-if="conversations.length === 0" class="pane-empty">
                No chats yet — start one above.
              </div>
              <button
                v-for="conv in conversations"
                :key="conv.id"
                class="conv-row"
                :class="{ 'conv-row--active': selectedConversationId === conv.id }"
                @click="selectConversation(conv.id)"
              >
                <span class="conv-title">{{ conv.title }}</span>
                <span class="conv-date">{{ formatDate(conv.updatedAt) }}</span>
              </button>
            </div>
          </template>
        </section>

        <!-- ── Chat panel ── -->
        <article class="chat-panel">
          <template v-if="selectedConversationId">
            <div class="chat-panel-header">
              <span class="chat-panel-title">
                {{ conversations.find((c) => c.id === selectedConversationId)?.title ?? 'Chat' }}
              </span>
              <span v-if="isGenerating" class="generating-dot" title="Generating" />
            </div>

            <MessageList
              :messages="messages"
              :pending-assistant="shouldShowPendingAssistantPlaceholder"
              :requeue-disabled="isSending"
              :theme="theme"
              @requeue="chatStore.handleRequeueMessage"
            />
            <p v-if="streamError" class="stream-error">{{ streamError }}</p>
            <EmptyChatGreeting
              v-if="shouldShowEmptyGreeting"
              :class="{ 'empty-chat-greeting--with-files': selectedFiles.length > 0 }"
            />
            <ChatComposer
              :draft="draft"
              :is-sending="isSending"
              :token-limit-reached="isConversationTokenCapReached"
              :selected-files="selectedFiles"
              @remove-file="chatStore.removeSelectedFile"
              @send="chatStore.sendMessage"
              @stop="chatStore.stopGeneration"
              @update-draft="chatStore.setDraft"
              @update-files="chatStore.handleSelectedFiles"
            />
          </template>

          <div v-else class="panel-empty">
            <AppIcon name="sparkles" :size="28" />
            <p>Select or create a chat</p>
          </div>
        </article>
      </div>
    </section>

    <NotebookCreateDialog
      v-if="showCreateDialog"
      @close="showCreateDialog = false"
      @created="handleCreate"
    />

    <NotebookSettingsDialog
      v-if="showSettingsDialog && selectedNotebook"
      :notebook="selectedNotebook"
      @close="showSettingsDialog = false"
      @saved="handleSaveSettings"
    />
  </div>
</template>

<style scoped>
.nb-view {
  display: grid;
  grid-template-columns: 280px 1fr;
  height: 100%;
  overflow: hidden;
  background: var(--bg);
  position: relative;
}

.nb-shell {
  min-width: 0;
  display: grid;
  grid-template-rows: auto 1fr;
  overflow: hidden;
}

.nb-reader {
  min-height: 0;
  display: grid;
  grid-template-columns: minmax(16rem, 22rem) minmax(0, 1fr);
  overflow: hidden;
}

/* ── Middle pane ── */
.middle-pane {
  border-right: 1px solid var(--border);
  display: grid;
  grid-template-rows: auto 1fr;
  overflow: hidden;
  min-width: 0;
}

.pane-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.75rem 0.9rem;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.pane-title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 720;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pane-subtitle {
  margin: 0.2rem 0 0;
  font-size: 0.78rem;
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pane-body {
  overflow-y: auto;
}

.pane-empty {
  padding: 1.5rem 0.9rem;
  font-size: 0.85rem;
  color: var(--muted);
  text-align: center;
}

.pane-error {
  color: var(--danger);
}

/* Files */
.upload-label {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.35rem 0.65rem;
  background: var(--primary);
  color: #fff;
  border: none;
  border-radius: 0.45rem;
  font-size: 0.82rem;
  font-weight: 650;
  cursor: pointer;
  flex-shrink: 0;
  white-space: nowrap;
}

.upload-label:hover {
  background: var(--primary-strong);
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
}

.upload-progress {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.9rem;
  position: relative;
  font-size: 0.78rem;
  color: var(--muted);
  border-bottom: 1px solid var(--border);
}

.upload-bar {
  position: absolute;
  inset: 0 auto 0 0;
  background: color-mix(in srgb, var(--primary) 10%, transparent);
  transition: width 0.2s;
}

.file-entry {
  border-bottom: 1px solid var(--border);
}

.file-entry:last-child {
  border-bottom: 0;
}

.file-row {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.52rem 0.65rem 0.52rem 0.9rem;
}

.file-name {
  flex: 1;
  font-size: 0.82rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-size {
  font-size: 0.72rem;
  color: var(--muted);
  flex-shrink: 0;
}

.file-status {
  font-size: 0.67rem;
  font-weight: 700;
  padding: 0.1rem 0.38rem;
  border-radius: 999px;
  flex-shrink: 0;
}

.file-status--pending,
.file-status--processing {
  background: color-mix(in srgb, var(--primary) 15%, transparent);
  color: var(--primary);
}

.file-status--ready {
  background: color-mix(in srgb, #22c55e 15%, transparent);
  color: #22c55e;
}

.file-status--error {
  background: color-mix(in srgb, var(--danger) 15%, transparent);
  color: var(--danger);
}

.file-err-btn {
  font-size: 0.68rem;
  font-weight: 800;
  color: var(--danger);
  background: color-mix(in srgb, var(--danger) 15%, transparent);
  border: 1px solid color-mix(in srgb, var(--danger) 30%, transparent);
  border-radius: 999px;
  width: 1.1rem;
  height: 1.1rem;
  display: inline-grid;
  place-items: center;
  cursor: pointer;
  flex-shrink: 0;
  padding: 0;
  line-height: 1;
}

.file-err-btn:hover,
.file-err-btn--open {
  background: color-mix(in srgb, var(--danger) 25%, transparent);
}

.file-del {
  width: 1.4rem;
  height: 1.4rem;
  display: inline-grid;
  place-items: center;
  border: 1px solid transparent;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  border-radius: 0.3rem;
  flex-shrink: 0;
}

.file-del:hover {
  color: var(--danger);
  background: var(--surface-hover);
}

.file-flyout {
  background: color-mix(in srgb, var(--danger) 6%, var(--surface));
  border-top: 1px solid color-mix(in srgb, var(--danger) 20%, transparent);
  padding: 0.45rem 0.9rem;
}

.file-flyout-msg {
  font-size: 0.76rem;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: ui-monospace, 'SFMono-Regular', monospace;
  color: var(--danger);
  opacity: 0.9;
  line-height: 1.45;
}

/* Conversations */
.conv-row {
  display: grid;
  gap: 0.2rem;
  padding: 0.65rem 0.9rem;
  border: 0;
  border-bottom: 1px solid var(--border);
  border-radius: 0;
  background: transparent;
  color: var(--text);
  text-align: left;
  cursor: pointer;
  width: 100%;
}

.conv-row:hover {
  background: var(--surface-hover);
}

.conv-row--active {
  background: var(--selected);
  border-left: 2px solid var(--primary);
  padding-left: calc(0.9rem - 2px);
}

.conv-title {
  font-size: 0.875rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
}

.conv-date {
  font-size: 0.74rem;
  color: var(--muted);
}

/* ── Chat panel ── */
.chat-panel {
  display: grid;
  grid-template-rows: auto 1fr auto;
  overflow: hidden;
  min-width: 0;
  background: var(--bg);
  position: relative;
}

.chat-panel-header {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.7rem 1rem;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}

.chat-panel-title {
  font-size: 0.9rem;
  font-weight: 650;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.generating-dot {
  width: 0.55rem;
  height: 0.55rem;
  border-radius: 999px;
  background: var(--primary);
  flex-shrink: 0;
  animation: blink 1.2s ease-in-out infinite;
}

@keyframes blink {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.25;
  }
}

.panel-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  height: 100%;
  color: color-mix(in srgb, var(--muted) 65%, transparent);
  font-size: 0.875rem;
}

.panel-empty p {
  margin: 0;
}

.stream-error {
  color: var(--danger);
  padding: 0 1.25rem 0.5rem;
  font-size: 0.875rem;
}

/* ── Shared buttons ── */
.header-actions {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.icon-btn {
  width: 2rem;
  height: 2rem;
  display: inline-grid;
  place-items: center;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--muted);
  cursor: pointer;
  border-radius: 0.45rem;
  flex-shrink: 0;
}

.icon-btn:hover {
  color: var(--text);
  background: var(--surface-hover);
}

.primary-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.38rem 0.75rem;
  background: var(--primary);
  color: #fff;
  border: none;
  border-radius: 0.45rem;
  font-size: 0.82rem;
  font-weight: 650;
  cursor: pointer;
  flex-shrink: 0;
  white-space: nowrap;
}

.primary-btn:hover {
  background: var(--primary-strong);
}

/* Progress flyout */
.file-status-btn {
  border: none;
  cursor: pointer;
  font-family: inherit;
  font-size: 0.67rem;
}

.file-status-btn:hover,
.file-status--open {
  filter: brightness(1.2);
}

.progress-flyout {
  background: color-mix(in srgb, var(--primary) 4%, var(--surface));
  border-top: 1px solid color-mix(in srgb, var(--primary) 18%, transparent);
  padding: 0.55rem 0.9rem 0.65rem;
}

.progress-status {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  margin-bottom: 0.5rem;
}

.progress-spinner {
  width: 0.65rem;
  height: 0.65rem;
  border-radius: 999px;
  border: 1.5px solid color-mix(in srgb, var(--primary) 30%, transparent);
  border-top-color: var(--primary);
  flex-shrink: 0;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.progress-status-text {
  font-size: 0.78rem;
  font-weight: 650;
  color: var(--primary);
  flex: 1;
}

.progress-page-count {
  font-size: 0.72rem;
  color: var(--primary);
  opacity: 0.75;
  white-space: nowrap;
}

.progress-bar-track {
  height: 4px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--primary) 18%, transparent);
  overflow: hidden;
  margin-bottom: 0.55rem;
}

.progress-bar-fill {
  height: 100%;
  border-radius: 999px;
  background: var(--primary);
  transition: width 0.6s ease;
  min-width: 4px;
}

.stage-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

.stage-item {
  display: flex;
  align-items: baseline;
  gap: 0.4rem;
  font-size: 0.77rem;
  color: var(--muted);
  line-height: 1.4;
}

.stage-dot {
  width: 0.32rem;
  height: 0.32rem;
  border-radius: 999px;
  background: color-mix(in srgb, var(--muted) 45%, transparent);
  flex-shrink: 0;
  position: relative;
  top: -0.05em;
}

.stage-note {
  font-size: 0.72rem;
  color: color-mix(in srgb, var(--muted) 65%, transparent);
  font-style: italic;
}

/* ── Responsive ── */
@media (max-width: 1100px) {
  .nb-reader {
    grid-template-columns: minmax(14rem, 18rem) minmax(0, 1fr);
  }
}

@media (max-width: 760px) {
  .nb-view {
    grid-template-columns: 1fr;
  }

  .nb-reader {
    grid-template-columns: 1fr;
    grid-template-rows: auto minmax(24rem, 1fr);
    overflow: auto;
  }

  .middle-pane {
    border-right: 0;
    border-bottom: 1px solid var(--border);
    overflow: visible;
  }

  .pane-body {
    max-height: 30vh;
    overflow: auto;
  }
}
</style>
