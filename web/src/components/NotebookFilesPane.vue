<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import type { Notebook, NotebookFile } from '../lib/notebooks'
import {
  formatBytes,
  formatElapsedSince,
  getProcessingStages,
  statusLabel,
} from '../lib/notebookFiles'
import AppIcon from './AppIcon.vue'

defineProps<{
  selectedNotebook: Notebook | null
  files: NotebookFile[]
  isUploading: boolean
  uploadProgress: number
  error: string | null
}>()

defineEmits<{
  upload: [event: Event]
  remove: [fileId: string]
}>()

const openErrorFileId = ref<string | null>(null)
const openProgressFileId = ref<string | null>(null)
const now = ref(Date.now())
let elapsedTimer: ReturnType<typeof setInterval> | null = null

function toggleErrorFlyout(fileId: string) {
  openErrorFileId.value = openErrorFileId.value === fileId ? null : fileId
  openProgressFileId.value = null
}

function toggleProgressFlyout(fileId: string) {
  openProgressFileId.value = openProgressFileId.value === fileId ? null : fileId
  openErrorFileId.value = null
}

onMounted(() => {
  elapsedTimer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (elapsedTimer !== null) {
    clearInterval(elapsedTimer)
    elapsedTimer = null
  }
})
</script>

<template>
  <div class="pane-header">
    <div>
      <h2 class="pane-title">Files</h2>
      <p v-if="selectedNotebook" class="pane-subtitle">{{ selectedNotebook.name }}</p>
    </div>
    <label v-if="selectedNotebook" class="upload-label" title="Upload file">
      <AppIcon name="plus" :size="14" />
      Upload
      <input type="file" multiple class="sr-only" @change="$emit('upload', $event)" />
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
        No files yet - upload one above.
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
            title="View processing stages"
            @click="toggleProgressFlyout(file.id)"
          >
            {{ statusLabel(file.status) }}
          </button>
          <span v-else class="file-status" :class="`file-status--${file.status}`">
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
          <button class="file-del" title="Remove" @click="$emit('remove', file.id)">
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
                  : `Processing... ${formatElapsedSince(file.updatedAt, now)}`
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
          <div v-if="file.status === 'processing' && file.pageCount > 0" class="progress-bar-track">
            <div
              class="progress-bar-fill"
              :style="{
                width: `${Math.round((file.pagesIndexed / file.pageCount) * 100)}%`,
              }"
            />
          </div>
          <ul class="stage-list">
            <li v-for="(stage, idx) in getProcessingStages(file)" :key="idx" class="stage-item">
              <span class="stage-dot" />
              <span class="stage-label">{{ stage.label }}</span>
              <span v-if="stage.note" class="stage-note">- {{ stage.note }}</span>
            </li>
          </ul>
        </div>
      </div>
      <p v-if="error" class="pane-empty pane-error">{{ error }}</p>
    </template>
  </div>
</template>

<style scoped>
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
</style>
