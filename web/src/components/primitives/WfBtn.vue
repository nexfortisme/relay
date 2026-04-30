<script setup lang="ts">
import type { CSSProperties } from 'vue'

defineProps<{
  primary?: boolean
  ghost?: boolean
  tiny?: boolean
  disabled?: boolean
  type?: 'button' | 'submit' | 'reset'
  style?: string | CSSProperties
}>()

const emit = defineEmits<{ click: [MouseEvent] }>()
</script>

<template>
  <button
    class="wf-btn"
    :class="{
      'wf-btn--primary': primary,
      'wf-btn--ghost': ghost,
      'wf-btn--tiny': tiny,
    }"
    :type="type ?? 'button'"
    :disabled="disabled"
    :style="style"
    @click="emit('click', $event)"
  >
    <slot />
  </button>
</template>

<style scoped>
.wf-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  padding: 0.45rem 0.75rem;
  border-radius: 0.45rem;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
  font-family: inherit;
}

.wf-btn:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--primary) 42%, var(--border));
  background: var(--surface-hover);
}

.wf-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.wf-btn--primary {
  border-color: color-mix(in srgb, var(--primary) 60%, transparent);
  background: var(--primary);
  color: #fff;
}

.wf-btn--primary:hover:not(:disabled) {
  background: var(--primary-strong);
  border-color: var(--primary-strong);
}

.wf-btn--ghost {
  border-color: transparent;
  background: transparent;
  color: var(--muted);
}

.wf-btn--ghost:hover:not(:disabled) {
  background: var(--surface-hover);
  color: var(--text);
}

.wf-btn--tiny {
  padding: 0.25rem 0.55rem;
  font-size: 0.75rem;
  border-radius: 0.35rem;
}
</style>
