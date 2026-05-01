<script setup lang="ts">
import { computed } from 'vue'

// PrismAvatar renders a small randomized cluster of triangular facets in the
// same visual language as the Relay prism logo. The shape regenerates on
// every render (no memoization, no seeding) per product requirement so each
// page load reads as a fresh facet of the same prism.
type Props = {
  size?: number
}
const props = withDefaults(defineProps<Props>(), { size: 32 })

const palette = [
  '#74d0f6',
  '#f1c27d',
  '#9ae6ff',
  '#f6a35d',
  '#c7f2ff',
  '#a78bfa',
  '#34d399',
]

function rand(min: number, max: number): number {
  return Math.random() * (max - min) + min
}

const facets = computed(() => {
  const count = 3 + Math.floor(Math.random() * 3) // 3–5 facets
  return Array.from({ length: count }, (_, idx) => ({
    rotation: rand(0, 360),
    skew: rand(-10, 10),
    offset: rand(-20, 20),
    color: palette[Math.floor(Math.random() * palette.length)],
    opacity: rand(0.55, 0.95),
    height: rand(60, 90),
    width: rand(20, 35),
    z: idx,
  }))
})

const baseRotation = computed(() => rand(0, 360))
const sizePx = computed(() => `${props.size}px`)
</script>

<template>
  <span class="prism-avatar" :style="{ width: sizePx, height: sizePx }" aria-hidden="true">
    <span class="prism-avatar__inner" :style="{ transform: `rotate(${baseRotation}deg)` }">
      <span
        v-for="facet in facets"
        :key="facet.z"
        class="prism-avatar__facet"
        :style="{
          borderLeftWidth: `${facet.width / 2}%`,
          borderRightWidth: `${facet.width / 2}%`,
          borderBottomWidth: `${facet.height}%`,
          borderBottomColor: facet.color,
          opacity: facet.opacity,
          transform: `translate(-50%, -100%) rotate(${facet.rotation}deg) skewX(${facet.skew}deg) translateY(${facet.offset}%)`,
        }"
      />
    </span>
  </span>
</template>

<style scoped>
.prism-avatar {
  position: relative;
  display: inline-block;
  border-radius: 50%;
  background: color-mix(in srgb, var(--primary) 14%, var(--surface));
  border: 1px solid color-mix(in srgb, var(--primary) 32%, var(--border));
  overflow: hidden;
  flex: 0 0 auto;
}

.prism-avatar__inner {
  position: absolute;
  inset: 0;
}

.prism-avatar__facet {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 0;
  height: 0;
  border-left-style: solid;
  border-right-style: solid;
  border-bottom-style: solid;
  border-left-color: transparent;
  border-right-color: transparent;
  transform-origin: 50% 100%;
  filter: drop-shadow(0 0.04rem 0.08rem color-mix(in srgb, currentColor 24%, transparent));
}
</style>
