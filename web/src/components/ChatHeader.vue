<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import AppIcon from './AppIcon.vue'

const props = defineProps<{
  isEditing: boolean
  isRenaming: boolean
  isSuggestingTitle: boolean
  renameDraft: string
  selectedConversationId: string | null
  title: string
  models?: string[]
  tokenCount?: number
  maxTokenCount?: number
}>()

const headerModels = computed(() => {
  const seen = new Set<string>()
  const models: string[] = []
  for (const model of props.models ?? []) {
    const normalized = model.trim()
    if (!normalized || seen.has(normalized)) {
      continue
    }
    seen.add(normalized)
    models.push(normalized)
  }
  return models
})
const hasTokenCap = computed(
  () => typeof props.maxTokenCount === 'number' && props.maxTokenCount > 0,
)
const displayTokenCount = computed(() => Math.max(0, props.tokenCount ?? 0))
const tokenMeterLabel = computed(() => {
  const used = formatTokenCount(displayTokenCount.value)
  if (!hasTokenCap.value) {
    return `${used} tokens`
  }
  return `${used} / ${formatTokenCount(props.maxTokenCount ?? 0)} tokens`
})
const tokenMeterPercent = computed(() => {
  if (!hasTokenCap.value) {
    return 0
  }
  return Math.min(100, (displayTokenCount.value / (props.maxTokenCount ?? 1)) * 100)
})
const isTokenCapReached = computed(
  () => hasTokenCap.value && displayTokenCount.value >= (props.maxTokenCount ?? 0),
)

function formatTokenCount(value: number): string {
  return Math.round(value).toLocaleString()
}

defineEmits<{
  archive: [event: MouseEvent]
  beginEdit: []
  cancelEdit: []
  saveTitle: []
  suggestTitle: []
  'update:renameDraft': [value: string]
}>()

const titleInputEl = ref<HTMLInputElement | null>(null)

watch(
  () => props.isEditing,
  async (isEditing) => {
    if (!isEditing) {
      return
    }
    await nextTick()
    titleInputEl.value?.focus()
    titleInputEl.value?.select()
  },
)
</script>

<template>
  <header class="chat-header">
    <div v-if="!isEditing" class="title-line">
      <div class="title-group">
        <h1 class="chat-title" :class="{ 'chat-title-suggesting': isSuggestingTitle }">
          {{ title }}
        </h1>
        <button
          class="title-icon-button"
          :disabled="isRenaming || isSuggestingTitle"
          title="Rename chat"
          aria-label="Rename chat"
          @click="$emit('beginEdit')"
        >
          <AppIcon name="pencil" :size="16" />
        </button>
        <button
          class="title-icon-button"
          :disabled="!selectedConversationId || isRenaming || isSuggestingTitle"
          title="Suggest title with AI"
          aria-label="Suggest title with AI"
          @click="$emit('suggestTitle')"
        >
          <AppIcon name="sparkles" :size="16" />
        </button>
      </div>
      <div class="header-trailing">
        <div v-if="headerModels.length" class="chat-model-list" aria-label="Models used in chat">
          <span
            v-for="model in headerModels"
            :key="model"
            class="chat-model-bubble"
            :title="`Model: ${model}`"
          >
            {{ model }}
          </span>
        </div>
        <div
          v-if="typeof tokenCount === 'number'"
          class="conversation-token-meter"
          :class="{ capped: isTokenCapReached }"
          :title="
            hasTokenCap
              ? `${tokenMeterLabel} used, excluding thinking tokens`
              : `${tokenMeterLabel}, excluding thinking tokens`
          "
        >
          <span>{{ tokenMeterLabel }}</span>
          <span v-if="hasTokenCap" class="conversation-token-bar" aria-hidden="true">
            <span :style="{ width: `${tokenMeterPercent}%` }" />
          </span>
        </div>
        <button
          class="header-action"
          :disabled="!selectedConversationId"
          title="Archive chat (Shift+click to delete)"
          @click="$emit('archive', $event)"
        >
          <AppIcon name="archive" :size="16" />
          <span class="header-action-label">Archive</span>
        </button>
      </div>
    </div>

    <div v-else class="title-edit-line">
      <input
        ref="titleInputEl"
        class="chat-title-input"
        :value="renameDraft"
        :disabled="isRenaming"
        @input="$emit('update:renameDraft', ($event.target as HTMLInputElement).value)"
        @blur="$emit('saveTitle')"
        @keydown.enter.prevent="$emit('saveTitle')"
        @keydown.esc.prevent="$emit('cancelEdit')"
      />
      <button
        class="title-icon-button"
        :disabled="isRenaming"
        title="Save title"
        @mousedown.prevent="$emit('saveTitle')"
      >
        <AppIcon name="check" :size="16" />
      </button>
      <button
        class="title-icon-button"
        title="Cancel rename"
        @mousedown.prevent="$emit('cancelEdit')"
      >
        <AppIcon name="x" :size="16" />
      </button>
    </div>
  </header>
</template>

<style scoped>
.chat-header {
  padding: 1rem 1.35rem;
  border-bottom: 1px solid var(--border);
  background: color-mix(in srgb, var(--bg) 88%, var(--surface));
}

.title-line,
.title-edit-line {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 0.75rem;
  min-height: 2.45rem;
}

.title-line .title-group {
  flex: 1 1 auto;
  min-width: 0;
}

.header-trailing {
  display: inline-flex;
  align-items: center;
  gap: 0.6rem;
  justify-content: flex-end;
  flex: 1 1 auto;
  min-width: 0;
  flex-wrap: wrap;
}

.chat-model-list {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.36rem;
  flex: 1 1 auto;
  min-width: 0;
  max-width: min(34rem, 42vw);
  flex-wrap: wrap;
}

.chat-model-bubble {
  display: inline-block;
  max-width: 13rem;
  min-height: 1.45rem;
  padding: 0.2rem 0.5rem;
  border: 1px solid color-mix(in srgb, var(--primary) 28%, var(--border));
  border-radius: 999px;
  background: color-mix(in srgb, var(--primary) 9%, transparent);
  color: var(--muted);
  font-size: 0.7rem;
  font-weight: 720;
  line-height: 1.1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conversation-token-meter {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  min-height: 1.7rem;
  padding: 0.28rem 0.55rem;
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  background: color-mix(in srgb, var(--surface) 92%, transparent);
  color: var(--muted);
  font-size: 0.74rem;
  font-variant-numeric: tabular-nums;
}

.conversation-token-meter.capped {
  color: var(--danger);
  border-color: color-mix(in srgb, var(--danger) 58%, var(--border));
}

.conversation-token-bar {
  width: 5.2rem;
  height: 0.34rem;
  border-radius: 999px;
  overflow: hidden;
  background: var(--surface-soft);
}

.conversation-token-bar span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--primary);
}

.conversation-token-meter.capped .conversation-token-bar span {
  background: var(--danger);
}

.title-group {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 0.42rem;
}

.chat-title {
  margin: 0;
  font-size: 1.22rem;
  font-weight: 720;
  line-height: 1.2;
  max-width: min(58vw, 760px);
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.chat-title-suggesting {
  background: linear-gradient(
    110deg,
    color-mix(in srgb, var(--text) 70%, #fff) 5%,
    color-mix(in srgb, var(--primary) 60%, #fff) 35%,
    #fff 50%,
    color-mix(in srgb, var(--primary) 60%, #fff) 65%,
    color-mix(in srgb, var(--text) 70%, #fff) 95%
  );
  background-size: 260% 100%;
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  filter: drop-shadow(0 0 0.4rem color-mix(in srgb, var(--primary) 35%, transparent));
  animation: title-shimmer 1s linear infinite;
}

@keyframes title-shimmer {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -20% 0;
  }
}

.title-icon-button,
.header-action {
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--muted);
  cursor: pointer;
  border-radius: 0.45rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.title-icon-button {
  width: 2rem;
  height: 2rem;
  flex: 0 0 auto;
}

.header-action {
  min-height: 2.2rem;
  gap: 0.42rem;
  padding: 0 0.72rem;
  font-weight: 650;
}

.title-icon-button:hover,
.header-action:hover {
  color: var(--text);
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
  background: var(--surface-hover);
}

.title-icon-button:disabled,
.header-action:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.title-edit-line {
  justify-content: flex-start;
}

.chat-title-input {
  font-size: 1.18rem;
  font-weight: 720;
  border: 1px solid var(--border);
  border-radius: 0.5rem;
  background: var(--surface);
  color: var(--text);
  outline: none;
  min-width: 220px;
  width: min(640px, 100%);
  padding: 0.48rem 0.62rem;
}

.chat-title-input:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary) 18%, transparent);
}

@media (max-width: 760px) {
  .chat-header {
    padding: 0.78rem 0.9rem;
  }

  .title-line {
    flex-wrap: wrap;
    align-items: flex-start;
    gap: 0.55rem;
  }

  .title-line .title-group {
    flex-basis: 100%;
  }

  .title-group {
    width: 100%;
  }

  .chat-title {
    max-width: none;
    font-size: 1.06rem;
  }

  .header-trailing {
    width: 100%;
    min-width: 0;
    justify-content: space-between;
  }

  .chat-model-list {
    order: 3;
    flex-basis: 100%;
    justify-content: flex-start;
    max-width: none;
  }

  .conversation-token-meter {
    flex: 1 1 auto;
    min-width: 0;
    justify-content: space-between;
  }

  .conversation-token-meter > span:first-child {
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  .conversation-token-bar {
    width: 3.8rem;
    flex: 0 0 auto;
  }

  .header-action {
    flex: 0 0 auto;
    padding-inline: 0.62rem;
  }

  .title-edit-line {
    gap: 0.45rem;
  }

  .chat-title-input {
    min-width: 0;
    font-size: 1.02rem;
  }
}

@media (max-width: 420px) {
  .conversation-token-bar,
  .header-action-label {
    display: none;
  }

  .header-action {
    width: 2.2rem;
    padding: 0;
  }
}
</style>
