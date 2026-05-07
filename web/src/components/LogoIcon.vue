<!--
  LogoIcon.vue — static V6 mark, pure CSS, no SVG/images.

  Props:
    size      Number  rendered px (square). Default 64.
    palette   Object  optional color overrides { a, b, c, core }
-->
<script setup lang="ts">
import { computed } from 'vue'

type Tone = 'a' | 'b' | 'c'

type Tri = {
  left: number
  top: number
  r: number
  s?: number
  tone: Tone
}

// Triangle placement in a 140×140 logo-space.
const TRIS: Tri[] = [
  { left: 60, top: 4, r: -10, tone: 'a' },
  { left: 82, top: 12, r: 35, tone: 'b' },
  { left: 96, top: 38, r: 75, tone: 'a' },
  { left: 98, top: 64, r: 110, tone: 'b' },
  { left: 84, top: 90, r: 150, tone: 'a' },
  { left: 60, top: 102, r: 180, tone: 'c' },
  { left: 36, top: 96, r: 215, tone: 'a' },
  { left: 16, top: 76, r: 250, tone: 'b' },
  { left: 8, top: 50, r: 285, tone: 'a' },
  { left: 18, top: 24, r: 320, tone: 'b' },
  { left: 38, top: 8, r: -30, tone: 'c' },
  { left: 50, top: 34, r: 15, s: 0.7, tone: 'c' },
  { left: 76, top: 56, r: 165, s: 0.7, tone: 'c' },
  { left: 44, top: 70, r: 245, s: 0.7, tone: 'c' },
]

type LogoPalette = {
  a: string
  b: string
  c: string
  core: string
}

const DEFAULT_PALETTE: LogoPalette = {
  a: '#8aa9d9',
  b: '#6f8fc4',
  c: '#b9cef0',
  core: '#0e1726',
}

const props = withDefaults(
  defineProps<{
    size?: number
    palette?: Partial<LogoPalette>
  }>(),
  {
    size: 64,
    palette: () => ({}),
  },
)

const palette = computed(() => ({ ...DEFAULT_PALETTE, ...props.palette }))
const scale = computed(() => props.size / 140)

const rootStyle = computed(() => ({
  '--logo-size': props.size + 'px',
  '--logo-core': palette.value.core,
}))

function posStyle(t: Tri) {
  const k = scale.value
  return {
    left: t.left * k + 'px',
    top: t.top * k + 'px',
    width: 22 * k + 'px',
    height: 30 * k + 'px',
    '--r': t.r + 'deg',
    '--s': t.s ?? 1,
    '--c': palette.value[t.tone],
  }
}
</script>

<template>
  <div class="v6-icon" :style="rootStyle" aria-hidden="true">
    <div v-for="(t, i) in TRIS" :key="i" class="pos" :style="posStyle(t)">
      <div class="tri" />
    </div>
    <div class="core" />
  </div>
</template>

<style scoped>
.v6-icon {
  width: var(--logo-size);
  height: var(--logo-size);
  position: relative;
  display: inline-block;
}
.pos {
  position: absolute;
  transform-origin: 50% 50%;
}
.tri {
  position: absolute;
  inset: 0;
  background: var(--c);
  clip-path: polygon(50% 0, 100% 100%, 0 100%);
  transform: rotate(var(--r, 0deg)) scale(var(--s, 1));
  transform-origin: 50% 50%;
}
.core {
  position: absolute;
  left: 50%;
  top: 50%;
  width: calc(var(--logo-size) * 0.085);
  height: calc(var(--logo-size) * 0.085);
  border-radius: 50%;
  background: var(--logo-core);
  transform: translate(-50%, -50%);
  z-index: 2;
}
</style>
