<script setup lang="ts">
import type { Settings } from '../lib/api'
import AppIcon from './AppIcon.vue'

const props = defineProps<{
  error: string
  saving: boolean
  settings: Settings
}>()

const emit = defineEmits<{
  close: []
  save: []
  'update:settings': [settings: Settings]
}>()

function updateSetting<K extends keyof Settings>(key: K, value: Settings[K]) {
  emit('update:settings', {
    ...props.settings,
    [key]: value,
  })
}
</script>

<template>
  <div class="settings-overlay" @click.self="$emit('close')">
    <div class="settings-panel">
      <div class="settings-header">
        <h2 class="settings-title">Settings</h2>
        <button class="settings-close" title="Close settings" @click="$emit('close')">
          <AppIcon name="x" :size="17" />
        </button>
      </div>
      <div class="settings-body">
        <label class="settings-label" for="llm-url">LLM URL</label>
        <input
          id="llm-url"
          class="settings-input"
          :value="settings.llm_url"
          placeholder="http://localhost:11434/v1"
          @input="updateSetting('llm_url', ($event.target as HTMLInputElement).value)"
        />
        <label class="settings-label" for="llm-model">Model</label>
        <input
          id="llm-model"
          class="settings-input"
          :value="settings.llm_model"
          placeholder="gpt-4o-mini"
          @input="updateSetting('llm_model', ($event.target as HTMLInputElement).value)"
        />
        <label class="settings-label" for="llm-api-key">LLM API Key</label>
        <input
          id="llm-api-key"
          class="settings-input"
          type="password"
          :value="settings.llm_api_key"
          placeholder="sk-..."
          autocomplete="off"
          @input="updateSetting('llm_api_key', ($event.target as HTMLInputElement).value)"
        />
        <label class="settings-label" for="system-prompt">System Prompt</label>
        <textarea
          id="system-prompt"
          class="settings-textarea"
          :value="settings.system_prompt"
          placeholder="You are a helpful assistant."
          rows="6"
          @input="updateSetting('system_prompt', ($event.target as HTMLTextAreaElement).value)"
        />
        <p v-if="error" class="settings-error">{{ error }}</p>
      </div>
      <div class="settings-footer">
        <button class="settings-cancel" @click="$emit('close')">Cancel</button>
        <button class="settings-save" :disabled="saving" @click="$emit('save')">
          {{ saving ? 'Saving...' : 'Save' }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-overlay {
  position: fixed;
  inset: 0;
  background: rgba(4, 9, 20, 0.62);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 1rem;
}

.settings-panel {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 0.75rem;
  width: min(500px, 100%);
  display: grid;
  grid-template-rows: auto 1fr auto;
  max-height: 85vh;
  overflow: hidden;
  box-shadow: var(--shadow);
}

.settings-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.15rem;
  border-bottom: 1px solid var(--border);
}

.settings-title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 720;
}

.settings-close {
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

.settings-close:hover {
  color: var(--text);
  background: var(--surface-hover);
  border-color: var(--border);
}

.settings-body {
  padding: 1.15rem;
  display: grid;
  gap: 0.55rem;
  overflow-y: auto;
}

.settings-label {
  font-size: 0.76rem;
  font-weight: 750;
  color: var(--muted);
  text-transform: uppercase;
  margin-top: 0.35rem;
}

.settings-input,
.settings-textarea {
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

.settings-input:focus,
.settings-textarea:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent);
}

.settings-textarea {
  resize: vertical;
  min-height: 108px;
}

.settings-error {
  color: var(--danger);
  font-size: 0.85rem;
  margin: 0;
}

.settings-footer {
  display: flex;
  gap: 0.6rem;
  justify-content: flex-end;
  padding: 1rem 1.15rem;
  border-top: 1px solid var(--border);
}

.settings-cancel,
.settings-save {
  min-height: 2.3rem;
  border-radius: 0.5rem;
  padding: 0 1rem;
  font-weight: 650;
  cursor: pointer;
}

.settings-cancel {
  border: 1px solid var(--border);
  background: var(--surface-soft);
  color: var(--text);
}

.settings-save {
  border: none;
  background: var(--primary);
  color: #fff;
}

.settings-save:hover {
  background: var(--primary-strong);
}

.settings-save:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

@media (max-width: 520px) {
  .settings-overlay {
    align-items: stretch;
    padding: 0.6rem;
  }

  .settings-panel {
    width: 100%;
    max-height: 100%;
    border-radius: 0.65rem;
  }

  .settings-header,
  .settings-body,
  .settings-footer {
    padding-inline: 0.9rem;
  }

  .settings-footer {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
}
</style>
