import { describe, expect, it } from 'vitest'
import { filenameFromDisposition } from './backup'

// covers: INV-BACKUP-05
describe('filenameFromDisposition', () => {
  const ref = new Date('2026-06-16T10:00:00Z')

  it('extracts a quoted filename', () => {
    expect(
      filenameFromDisposition('attachment; filename="household-backup-2026-01-02.json.gz"'),
    ).toBe('household-backup-2026-01-02.json.gz')
  })

  it('extracts an unquoted filename', () => {
    expect(
      filenameFromDisposition('attachment; filename=household-backup-2026-01-02.json.gz'),
    ).toBe('household-backup-2026-01-02.json.gz')
  })

  it('handles the RFC 5987 filename* form', () => {
    expect(
      filenameFromDisposition("attachment; filename*=UTF-8''household-backup-2026-01-02.json.gz"),
    ).toBe('household-backup-2026-01-02.json.gz')
  })

  it('falls back to a date-stamped default when the header is null', () => {
    expect(filenameFromDisposition(null, ref)).toBe('household-backup-2026-06-16.json.gz')
  })

  it('falls back when the header has no filename', () => {
    expect(filenameFromDisposition('attachment', ref)).toBe('household-backup-2026-06-16.json.gz')
  })
})
