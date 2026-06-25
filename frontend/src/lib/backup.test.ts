import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/client'
import {
  filenameFromDisposition,
  isHouseholdEmpty,
  postRestoreCommit,
  postRestorePreview,
  totalRows,
} from './backup'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

function backupFile(): File {
  return new File([new Uint8Array([0x1f, 0x8b])], 'household-backup.json.gz')
}

afterEach(() => {
  vi.restoreAllMocks()
})

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

// covers: INV-BACKUP-10
describe('restore stakes helpers', () => {
  it('totalRows sums every section count', () => {
    expect(totalRows({ assets: 3, asset_snapshots: 12, income: 1 })).toBe(16)
    expect(totalRows({})).toBe(0)
  })

  it('isHouseholdEmpty is true only when every section is zero', () => {
    expect(isHouseholdEmpty({})).toBe(true)
    expect(isHouseholdEmpty({ assets: 0, income: 0 })).toBe(true)
    expect(isHouseholdEmpty({ assets: 0, income: 1 })).toBe(false)
  })
})

// covers: INV-BACKUP-08
describe('postRestore data layer', () => {
  it('preview uploads the file as multipart to the preview endpoint', async () => {
    const preview = {
      backup: { household_name: 'Home', format_version: 1, source_format_version: 1, fidelity: 'full', counts: { assets: 1 } },
      current: { assets: 0 },
    }
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, preview))
    vi.stubGlobal('fetch', fetchMock)

    const res = await postRestorePreview(backupFile())
    expect(res).toEqual(preview)

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/backup/restore/preview')
    expect(init.method).toBe('POST')
    expect(init.body).toBeInstanceOf(FormData)
    expect((init.body as FormData).get('file')).toBeInstanceOf(File)
  })

  it('commit posts to the commit endpoint', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, { restored: true, summary: { counts: {} } }))
    vi.stubGlobal('fetch', fetchMock)

    const res = await postRestoreCommit(backupFile())
    expect(res.restored).toBe(true)
    expect(fetchMock.mock.calls[0][0]).toBe('/api/backup/restore/commit')
  })

  it('throws an ApiError carrying the ADR-0027 envelope on a non-2xx', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse(403, { code: 'NOT_A_MEMBER_OF_BACKUP' })),
    )

    const err = await postRestorePreview(backupFile()).catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(403)
    expect((err as ApiError).body).toEqual({ code: 'NOT_A_MEMBER_OF_BACKUP' })
  })

  it('drops a non-envelope JSON error body (no code field)', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(500, { oops: true })))

    const err = (await postRestoreCommit(backupFile()).catch((e) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(500)
    // Not an ADR-0027 envelope → body is left undefined so errorMessage() falls
    // through to the generic copy rather than surfacing a stray shape.
    expect(err.body).toBeUndefined()
  })

  it('captures a non-JSON error body as raw text (#185)', async () => {
    // A proxy/gateway can answer a non-2xx with HTML, not the JSON envelope.
    // The old code read res.json() first (consuming the stream), so the text
    // fallback came back empty and the body was lost; now it's surfaced.
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response('<html>502 Bad Gateway</html>', {
          status: 502,
          headers: { 'Content-Type': 'text/html' },
        }),
      ),
    )

    const err = (await postRestoreCommit(backupFile()).catch((e) => e)) as ApiError
    expect(err).toBeInstanceOf(ApiError)
    expect(err.status).toBe(502)
    expect(err.body).toBe('<html>502 Bad Gateway</html>')
  })
})
