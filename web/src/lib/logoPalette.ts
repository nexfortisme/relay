/**
 * Theme-aware V6 logo colors for LogoIcon / LogoLoader.
 * Uses CSS values so the mark tracks `--primary` / `--surface`.
 */
export const brandLogoPalette = {
  a: 'var(--primary)',
  b: 'color-mix(in srgb, var(--primary) 78%, #ffffff 22%)',
  c: 'color-mix(in srgb, var(--primary) 60%, #ffffff 40%)',
  core: 'var(--surface)',
} as const

export const loaderPalette = brandLogoPalette
