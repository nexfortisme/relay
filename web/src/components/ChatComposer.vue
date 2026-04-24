<script setup lang="ts">
import { ref, watch } from 'vue'
import AppIcon from './AppIcon.vue'

const props = defineProps<{
  draft: string
  isSending: boolean
  selectedFiles: File[]
}>()

const emit = defineEmits<{
  removeFile: [index: number]
  send: []
  stop: []
  updateDraft: [value: string]
  updateFiles: [files: File[]]
}>()

const fileInputEl = ref<HTMLInputElement | null>(null)

watch(
  () => props.selectedFiles.length,
  (fileCount) => {
    if (fileCount === 0 && fileInputEl.value) {
      fileInputEl.value.value = ''
    }
  },
)

function openFilePicker() {
  fileInputEl.value?.click()
}

function handleFileSelection(event: Event) {
  const input = event.target as HTMLInputElement
  const files = input.files ? Array.from(input.files) : []
  emit('updateFiles', files)
}
</script>

<template>
  <form class="composer" @submit.prevent="$emit('send')">
    <div v-if="selectedFiles.length" class="file-list">
      <span v-for="(file, index) in selectedFiles" :key="`${file.name}-${index}`" class="file-chip">
        <span class="file-chip-label">{{ file.name }}</span>
        <button type="button" class="file-chip-remove" title="Remove file" @click="$emit('removeFile', index)">
          <AppIcon name="x" :size="13" />
        </button>
      </span>
    </div>
    <button
      type="button"
      class="file-picker-button"
      aria-label="Upload files"
      title="Upload files"
      @click="openFilePicker"
    >
      <AppIcon name="paperclip" :size="18" />
    </button>
    <input
      ref="fileInputEl"
      class="file-picker-hidden"
      type="file"
      multiple
      accept="image/*,.pdf,.txt,.md,.markdown,.json,.csv,.xml,.yaml,.yml"
      @change="handleFileSelection"
    />
    <input
      class="composer-input"
      :value="draft"
      placeholder="Ask something..."
      @input="$emit('updateDraft', ($event.target as HTMLInputElement).value)"
    />
    <button
      :type="isSending ? 'button' : 'submit'"
      :disabled="!isSending && !draft.trim()"
      class="composer-send-button"
      :class="{ 'stop-button': isSending }"
      :title="isSending ? 'Stop generation' : 'Send message'"
      @click="isSending ? $emit('stop') : undefined"
    >
      <AppIcon :name="isSending ? 'square' : 'send'" :size="17" />
      <span>{{ isSending ? 'Stop' : 'Send' }}</span>
    </button>
  </form>
</template>

<style scoped>
.composer {
  position: absolute;
  left: 1.35rem;
  right: 1.35rem;
  bottom: 1rem;
  padding: 0.62rem;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  grid-template-rows: auto auto;
  grid-template-areas:
    'files files files'
    'upload input send';
  gap: 0.58rem;
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  background: color-mix(in srgb, var(--surface) 94%, transparent);
  box-shadow: var(--shadow);
  backdrop-filter: blur(18px);
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
  min-width: 0;
  max-width: 18rem;
  gap: 0.35rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  padding: 0.22rem 0.28rem 0.22rem 0.5rem;
  font-size: 0.76rem;
}

.file-chip-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-chip-remove {
  width: 1.35rem;
  height: 1.35rem;
  border: none;
  border-radius: 999px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  display: inline-grid;
  place-items: center;
}

.file-chip-remove:hover {
  background: var(--surface-hover);
  color: var(--text);
}

.file-picker-button,
.composer-send-button {
  border: none;
  background: var(--primary);
  color: #fff;
  font-weight: 650;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.file-picker-button {
  grid-area: upload;
  width: 2.65rem;
  min-width: 2.65rem;
  border-radius: 0.55rem;
}

.file-picker-hidden {
  display: none;
}

.composer-input {
  grid-area: input;
  min-width: 0;
  padding: 0.76rem 0.85rem;
  border-radius: 0.55rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  outline: none;
  font: inherit;
}

.composer-input:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent);
}

.composer-send-button {
  grid-area: send;
  min-width: 5.2rem;
  gap: 0.42rem;
  border-radius: 0.55rem;
  padding: 0 0.92rem;
}

.file-picker-button:hover,
.composer-send-button:hover {
  background: var(--primary-strong);
}

.stop-button {
  background: var(--danger);
}

.stop-button:hover {
  background: var(--danger-strong);
}

.composer-send-button:disabled {
  opacity: 0.55;
  cursor: not-allowed;
  background: var(--primary-strong);
}
</style>
