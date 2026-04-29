<template>
  <span class="prism-logo" aria-hidden="true">
    <span class="prism-logo__inner">
      <span
        v-for="index in 4"
        :key="index"
        class="prism-logo__facet"
        :style="{ '--facet-index': index - 1 }"
      />
    </span>
  </span>
</template>

<style scoped>
.prism-logo {
  --logo-size: 3.1rem;
  --logo-face-w: 0.68rem;
  --logo-face-h: 1.34rem;
  position: relative;
  display: inline-block;
  width: var(--logo-size);
  height: var(--logo-size);
  flex: 0 0 auto;
  color: var(--primary);
}

.prism-logo__inner {
  position: absolute;
  inset: 0;
  transform: rotate(45deg);
}

.prism-logo__facet {
  position: absolute;
  top: 50%;
  left: 50%;
  width: 0;
  height: 0;
  border-left: var(--logo-face-w) solid transparent;
  border-right: var(--logo-face-w) solid transparent;
  border-bottom: var(--logo-face-h) solid color-mix(in srgb, currentColor 80%, #74d0f6);
  filter: drop-shadow(0 0.18rem 0.36rem color-mix(in srgb, currentColor 22%, transparent));
  opacity: 0.86;
  transform:
    translate(-50%, -100%)
    rotate(calc(var(--facet-index) * 90deg))
    translateY(-0.18rem);
  transform-origin: 50% 100%;
}

.prism-logo__facet:nth-child(1) {
  border-bottom-color: color-mix(in srgb, currentColor 72%, #f1c27d);
}

.prism-logo__facet:nth-child(2) {
  border-bottom-color: color-mix(in srgb, currentColor 78%, #9ae6ff);
  opacity: 0.7;
}

.prism-logo__facet:nth-child(3) {
  border-bottom-color: color-mix(in srgb, currentColor 66%, #f6a35d);
  opacity: 0.78;
}

.prism-logo__facet:nth-child(4) {
  border-bottom-color: color-mix(in srgb, currentColor 84%, #c7f2ff);
  opacity: 0.64;
}
</style>
