<script setup lang="ts">
import { computed } from 'vue'

// PrismAvatar renders a chaotic cluster of triangular shards in the same
// visual language as the Relay prism logo. One palette hue is picked per
// render and each shard is shaded in variations of that hue. Regenerates on
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

const avatar = computed(() => {
  const size = props.size
  const baseColor = palette[Math.floor(Math.random() * palette.length)]
  const baseRotation = rand(0, 360)
  const count = 9 + Math.floor(Math.random() * 5) // 9–13 shards
  const facets = Array.from({ length: count }, (_, idx) => {
    // shade < 0 mixes toward shadow, shade > 0 mixes toward highlight
    const shade = rand(-55, 45)
    const color =
      shade < 0
        ? `color-mix(in srgb, ${baseColor} ${100 + shade}%, #161821)`
        : `color-mix(in srgb, ${baseColor} ${100 - shade}%, #ffffff)`
    return {
      rotation: rand(0, 360),
      skew: rand(-18, 18),
      // Outward push along the shard's pointing axis. Mostly positive so
      // shards radiate outward; small negative range keeps the bases
      // overlapping in the middle for cluster density.
      outward: rand(-0.06, 0.22) * size,
      halfWidth: Math.max(1.5, rand(0.06, 0.16) * size),
      height: Math.max(8, rand(0.34, 0.58) * size),
      color,
      opacity: rand(0.62, 0.96),
      z: idx,
    }
  })
  return { baseColor, baseRotation, facets }
})

const sizePx = computed(() => `${props.size}px`)
</script>

<template>
  <span
    class="prism-avatar"
    :style="{
      width: sizePx,
      height: sizePx,
      '--avatar-base': avatar.baseColor,
    }"
    aria-hidden="true"
  >
    <span class="prism-avatar__inner" :style="{ transform: `rotate(${avatar.baseRotation}deg)` }">
      <span
        v-for="facet in avatar.facets"
        :key="facet.z"
        class="prism-avatar__facet"
        :style="{
          borderLeftWidth: `${facet.halfWidth}px`,
          borderRightWidth: `${facet.halfWidth}px`,
          borderBottomWidth: `${facet.height}px`,
          borderBottomColor: facet.color,
          opacity: facet.opacity,
          transform: `translate(-50%, -100%) rotate(${facet.rotation}deg) skewX(${facet.skew}deg) translateY(${-facet.outward}px)`,
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
  background: color-mix(in srgb, var(--avatar-base) 16%, var(--surface));
  border: 1px solid color-mix(in srgb, var(--avatar-base) 38%, var(--border));
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
  filter: drop-shadow(0 0.04rem 0.08rem color-mix(in srgb, currentColor 28%, transparent));
}
</style>
