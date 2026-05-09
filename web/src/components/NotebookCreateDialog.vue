<script setup lang="ts">
import { ref } from 'vue'
import { formatBytes } from '../lib/notebookFiles'
import AppIcon from './AppIcon.vue'

const emit = defineEmits<{
  close: []
  created: [payload: { name: string; description: string; systemPrompt: string; includeInGeneral: boolean; files: File[] }]
}>()

const name = ref('')
const description = ref('')
const systemPrompt = ref('')
const includeInGeneral = ref(false)
const selectedFiles = ref<File[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const nameError = ref('')

function triggerFilePicker() {
  fileInput.value?.click()
}

function onFilesSelected(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files) return
  const added = Array.from(input.files)
  const existing = new Set(selectedFiles.value.map((f) => f.name))
  for (const f of added) {
    if (!existing.has(f.name)) {
      selectedFiles.value.push(f)
      existing.add(f.name)
    }
  }
  input.value = ''
}

function removeFile(index: number) {
  selectedFiles.value.splice(index, 1)
}

function submit() {
  nameError.value = ''
  if (!name.value.trim()) {
    nameError.value = 'Name is required'
    return
  }
  emit('created', {
    name: name.value.trim(),
    description: description.value.trim(),
    systemPrompt: systemPrompt.value.trim(),
    includeInGeneral: includeInGeneral.value,
    files: [...selectedFiles.value],
  })
}
</script>

<template>
  <div class="dialog-overlay" @click.self="$emit('close')">
    <div class="dialog-panel" role="dialog" aria-modal="true" aria-label="Create notebook">
      <div class="dialog-header">
        <h2 class="dialog-title">New Notebook</h2>
        <button class="dialog-close" title="Close" @click="$emit('close')">
          <AppIcon name="x" :size="17" />
        </button>
      </div>

      <div class="dialog-body">
        <label class="field-label" for="nb-name">Name</label>
        <input
          id="nb-name"
          v-model="name"
          class="field-input"
          :class="{ 'field-input--error': nameError }"
          placeholder="My Notebook"
          autofocus
          @keydown.enter="submit"
        />
        <p v-if="nameError" class="field-error">{{ nameError }}</p>

        <label class="field-label" for="nb-description">Description</label>
        <input
          id="nb-description"
          v-model="description"
          class="field-input"
          placeholder="Optional description"
        />

        <label class="field-label" for="nb-prompt">System Prompt</label>
        <textarea
          id="nb-prompt"
          v-model="systemPrompt"
          class="field-textarea"
          placeholder="Instructions for the LLM when chatting within this notebook. Replaces your global system prompt."
          rows="3"
        />

        <label class="toggle-row" for="nb-include-general">
          <div class="toggle-text">
            <span class="toggle-label">Show chats in general Chat view</span>
            <span class="toggle-hint">Conversations from this notebook will appear alongside regular chats</span>
          </div>
          <div class="toggle-switch" :class="{ active: includeInGeneral }">
            <input
              id="nb-include-general"
              v-model="includeInGeneral"
              type="checkbox"
              class="sr-only"
            />
            <div class="toggle-thumb" />
          </div>
        </label>

        <div class="field-label-row">
          <label class="field-label">Files</label>
          <button class="add-files-btn" type="button" @click="triggerFilePicker">
            <AppIcon name="plus" :size="14" />
            Add files
          </button>
        </div>
        <input
          ref="fileInput"
          type="file"
          multiple
          class="sr-only"
          @change="onFilesSelected"
        />
        <div v-if="selectedFiles.length > 0" class="file-list">
          <div v-for="(file, i) in selectedFiles" :key="file.name" class="file-row">
            <span class="file-name">{{ file.name }}</span>
            <span class="file-size">{{ formatBytes(file.size) }}</span>
            <button class="file-remove" type="button" title="Remove" @click="removeFile(i)">
              <AppIcon name="x" :size="13" />
            </button>
          </div>
        </div>
        <p v-else class="field-hint">PDF, CSV, Markdown, images, JSON, YAML — up to 512 MB each</p>
      </div>

      <div class="dialog-footer">
        <button class="btn-cancel" @click="$emit('close')">Cancel</button>
        <button class="btn-create" @click="submit">Create</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dialog-overlay {
  position: fixed;
  inset: 0;
  background: rgba(4, 9, 20, 0.62);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 1rem;
}

.dialog-panel {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  width: min(520px, 100%);
  display: grid;
  grid-template-rows: auto 1fr auto;
  max-height: 90vh;
  overflow: hidden;
  box-shadow: var(--shadow);
}

.dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.15rem;
  border-bottom: 1px solid var(--border);
}

.dialog-title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 720;
}

.dialog-close {
  width: 2rem;
  height: 2rem;
  border: 1px solid transparent;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  border-radius: 0.45rem;
  display: inline-grid;
  place-items: center;
}

.dialog-close:hover {
  color: var(--text);
  background: var(--surface-hover);
  border-color: var(--border);
}

.dialog-body {
  padding: 1.15rem;
  display: grid;
  gap: 0.5rem;
  overflow-y: auto;
  align-content: start;
}

.field-label {
  font-size: 0.76rem;
  font-weight: 750;
  color: var(--muted);
  text-transform: uppercase;
  margin-top: 0.35rem;
}

.field-label-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 0.35rem;
}

.field-label-row .field-label {
  margin-top: 0;
}

.field-input,
.field-textarea {
  padding: 0.68rem 0.78rem;
  border-radius: 0.55rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  font-size: 0.92rem;
  font-family: inherit;
  width: 100%;
  box-sizing: border-box;
  outline: none;
}

.field-input:focus,
.field-textarea:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent);
}

.field-input--error {
  border-color: var(--danger);
}

.field-textarea {
  resize: vertical;
  min-height: 100px;
}

.field-error {
  color: var(--danger);
  font-size: 0.82rem;
  margin: 0;
}

.field-hint {
  color: var(--muted);
  font-size: 0.8rem;
  margin: 0;
}

.toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.65rem 0.75rem;
  border-radius: 0.55rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  cursor: pointer;
  margin-top: 0.35rem;
}

.toggle-row:hover {
  background: var(--surface-hover);
}

.toggle-text {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 0;
}

.toggle-label {
  font-size: 0.88rem;
  font-weight: 600;
  color: var(--text);
}

.toggle-hint {
  font-size: 0.78rem;
  color: var(--muted);
  line-height: 1.35;
}

.toggle-switch {
  flex-shrink: 0;
  width: 2.4rem;
  height: 1.35rem;
  border-radius: 999px;
  background: var(--border);
  position: relative;
  transition: background 0.18s;
}

.toggle-switch.active {
  background: var(--primary);
}

.toggle-thumb {
  position: absolute;
  top: 0.18rem;
  left: 0.18rem;
  width: 1rem;
  height: 1rem;
  border-radius: 999px;
  background: #fff;
  transition: transform 0.18s;
  box-shadow: 0 1px 3px rgba(0,0,0,0.2);
}

.toggle-switch.active .toggle-thumb {
  transform: translateX(1.05rem);
}

.add-files-btn {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.3rem 0.65rem;
  border-radius: 0.4rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
}

.add-files-btn:hover {
  background: var(--surface-hover);
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
}

.file-list {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.file-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.45rem 0.65rem;
  border-radius: 0.45rem;
  border: 1px solid var(--border);
  background: var(--surface-soft);
}

.file-name {
  flex: 1;
  font-size: 0.85rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-size {
  font-size: 0.78rem;
  color: var(--muted);
  white-space: nowrap;
}

.file-remove {
  width: 1.5rem;
  height: 1.5rem;
  display: inline-grid;
  place-items: center;
  border: none;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
  border-radius: 0.3rem;
  flex-shrink: 0;
}

.file-remove:hover {
  color: var(--danger);
  background: var(--surface-hover);
}

.dialog-footer {
  display: flex;
  gap: 0.6rem;
  justify-content: flex-end;
  padding: 1rem 1.15rem;
  border-top: 1px solid var(--border);
}

.btn-cancel,
.btn-create {
  min-height: 2.3rem;
  border-radius: 0.5rem;
  padding: 0 1rem;
  font-weight: 650;
  cursor: pointer;
  font-size: 0.9rem;
}

.btn-cancel {
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
}

.btn-create {
  border: none;
  background: var(--primary);
  color: #fff;
}

.btn-create:hover {
  background: var(--primary-strong);
}

@media (max-width: 520px) {
  .dialog-overlay {
    align-items: flex-end;
    padding: 0;
  }

  .dialog-panel {
    width: 100%;
    border-radius: 0.75rem 0.75rem 0 0;
    max-height: 92vh;
  }
}
</style>
