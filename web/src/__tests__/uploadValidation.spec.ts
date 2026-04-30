import { describe, expect, it } from 'vitest'
import {
  formatBytesLabel,
  parsePositiveInt,
  shouldAlertUploadFailure,
  validateSelectedFiles,
  type UploadLimits,
} from '../lib/uploadValidation'

const limits: UploadLimits = {
  maxSingleFileBytes: 10,
  maxTotalUploadBytes: 18,
  maxImageUploadBytes: 6,
}

function file(name: string, size: number, type = 'text/plain'): File {
  return new File(['x'.repeat(size)], name, { type })
}

describe('uploadValidation', () => {
  it('parses positive integer env values with fallback handling', () => {
    expect(parsePositiveInt('12', 5)).toBe(12)
    expect(parsePositiveInt('0', 5)).toBe(5)
    expect(parsePositiveInt('nope', 5)).toBe(5)
    expect(parsePositiveInt(undefined, 5)).toBe(5)
  })

  it('formats byte labels for user-facing upload messages', () => {
    expect(formatBytesLabel(512)).toBe('512B')
    expect(formatBytesLabel(1536)).toBe('1.5KB')
    expect(formatBytesLabel(2 * 1024 * 1024)).toBe('2MB')
  })

  it('validates single file, total upload, and image limits', () => {
    expect(validateSelectedFiles([file('huge.txt', 11)], limits)).toContain(
      'Files must be 10B or smaller: huge.txt',
    )
    expect(validateSelectedFiles([file('a.txt', 9), file('b.txt', 10)], limits)).toContain(
      '18B total upload limit',
    )
    expect(validateSelectedFiles([file('photo.png', 7, 'image/png')], limits)).toContain(
      'Image files must be 6B or smaller: photo.png',
    )
    expect(validateSelectedFiles([file('ok.txt', 4)], limits)).toBeNull()
  })

  it('only alerts upload failures when files were attached', () => {
    expect(shouldAlertUploadFailure('payload too large', [file('a.txt', 1)])).toBe(true)
    expect(shouldAlertUploadFailure('payload too large', [])).toBe(false)
    expect(shouldAlertUploadFailure('plain network hiccup', [file('a.txt', 1)])).toBe(false)
  })
})
