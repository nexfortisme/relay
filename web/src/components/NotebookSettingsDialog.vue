<script setup lang="ts">
import { ref } from 'vue'
import AppIcon from './AppIcon.vue'
import type { Notebook } from '../lib/notebooks'

const props = defineProps<{ notebook: Notebook }>()

const emit = defineEmits<{
  close: []
  saved: [payload: Partial<{ name: string; description: string; systemPrompt: string; skillPrompt: string }>]
}>()

const name = ref(props.notebook.name)
const description = ref(props.notebook.description)
const systemPrompt = ref(props.notebook.systemPrompt)
const skillPrompt = ref(props.notebook.skillPrompt)
const nameError = ref('')

function submit() {
  nameError.value = ''
  if (!name.value.trim()) {
    nameError.value = 'Name is required'
    return
  }
  emit('saved', {
    name: name.value.trim(),
    description: description.value.trim(),
    systemPrompt: systemPrompt.value.trim(),
    skillPrompt: skillPrompt.value.trim(),
  })
}
</script>

<template>
  <div class="dialog-overlay" @click.self="$emit('close')">
    <div class="dialog-panel" role="dialog" aria-modal="true" aria-label="Notebook settings">
      <div class="dialog-header">
        <h2 class="dialog-title">Notebook Settings</h2>
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

        <label class="field-label" for="nb-system-prompt">System Prompt</label>
        <textarea
          id="nb-system-prompt"
          v-model="systemPrompt"
          class="field-textarea"
          placeholder="Instructions for the LLM when chatting within this notebook. Replaces your global system prompt."
          rows="4"
        />

        <label class="field-label" for="nb-skill-prompt">Skill Prompt</label>
        <textarea
          id="nb-skill-prompt"
          v-model="skillPrompt"
          class="field-textarea"
          placeholder="Additional instructions injected alongside retrieved document context. Use this to shape how the LLM responds to document queries — e.g. &quot;Always cite the exact page number&quot; or &quot;Focus on technical accuracy.&quot;"
          rows="4"
        />
      </div>

      <div class="dialog-footer">
        <button class="btn-cancel" @click="$emit('close')">Cancel</button>
        <button class="btn-save" @click="submit">Save</button>
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
  min-height: 90px;
}

.field-error {
  color: var(--danger);
  font-size: 0.82rem;
  margin: 0;
}

.dialog-footer {
  display: flex;
  gap: 0.6rem;
  justify-content: flex-end;
  padding: 1rem 1.15rem;
  border-top: 1px solid var(--border);
}

.btn-cancel,
.btn-save {
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

.btn-save {
  border: none;
  background: var(--primary);
  color: #fff;
}

.btn-save:hover {
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
