<script setup lang="ts">
const props = defineProps<{ on?: boolean; modelValue?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [boolean] }>()

const checked = () => props.modelValue ?? props.on ?? false

function toggle() {
  if (props.modelValue !== undefined) {
    emit('update:modelValue', !props.modelValue)
  }
}
</script>

<template>
  <span
    class="wf-cb"
    :class="{ 'wf-cb--on': checked() }"
    role="checkbox"
    :aria-checked="checked()"
    tabindex="0"
    @click="toggle"
    @keydown.space.prevent="toggle"
  >
    <svg v-if="checked()" viewBox="0 0 24 24" fill="none" stroke="currentColor"
         stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
      <path d="M20 6 9 17l-5-5" />
    </svg>
  </span>
</template>

<style scoped>
.wf-cb {
  display: inline-grid;
  place-items: center;
  width: 0.95rem;
  height: 0.95rem;
  border-radius: 0.25rem;
  border: 1.5px solid var(--border);
  background: var(--surface);
  cursor: pointer;
  flex: 0 0 auto;
}

.wf-cb:focus-visible {
  outline: 2px solid color-mix(in srgb, var(--primary) 55%, transparent);
  outline-offset: 1px;
}

.wf-cb--on {
  background: var(--primary);
  border-color: var(--primary);
  color: #fff;
}

.wf-cb svg {
  width: 0.7rem;
  height: 0.7rem;
}
</style>
