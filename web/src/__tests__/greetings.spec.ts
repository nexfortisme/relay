import { describe, expect, it } from 'vitest'
import {
  greetingPeriodFor,
  parseGreetingMarkdown,
  selectGreeting,
} from '../lib/greetings'

const greetingMarkdown = `
# Morning

- Morning

# Afternoon

- Afternoon

# Evening

- Evening

# Night

- Night
`

describe('greetings', () => {
  it('maps local hours to greeting periods', () => {
    expect(greetingPeriodFor(new Date(2026, 3, 29, 6))).toBe('morning')
    expect(greetingPeriodFor(new Date(2026, 3, 29, 13))).toBe('afternoon')
    expect(greetingPeriodFor(new Date(2026, 3, 29, 18))).toBe('evening')
    expect(greetingPeriodFor(new Date(2026, 3, 29, 22))).toBe('night')
    expect(greetingPeriodFor(new Date(2026, 3, 29, 3))).toBe('night')
  })

  it('parses markdown sections into time-of-day lists', () => {
    expect(parseGreetingMarkdown(greetingMarkdown)).toEqual({
      morning: ['Morning'],
      afternoon: ['Afternoon'],
      evening: ['Evening'],
      night: ['Night'],
    })
  })

  it('selects a period greeting without adding a name', () => {
    expect(selectGreeting(greetingMarkdown, new Date(2026, 3, 29, 13))).toBe('Afternoon')
  })
})
