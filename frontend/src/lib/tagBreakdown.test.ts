import { describe, it, expect } from 'vitest'
import { aggregateTagBreakdown } from '@/lib/tagBreakdown'
import { UNTAGGED_COLOR } from '@/lib/tagColors'
import type { Tag, TagBreakdownRow } from '@/api/types'

const tag = (id: string, name: string, color: string): Tag => ({
  id,
  household_id: 'h1',
  name,
  color,
  created_by: null,
  created_at: '',
  updated_by: null,
  updated_at: '',
  deleted_at: null,
})

const row = (
  tag_id: string | null,
  group: TagBreakdownRow['group'],
  currency: string,
  total: string,
): TagBreakdownRow => ({ tag_id, group, currency, total })

const tags = [tag('t1', 'BCA', '#3b82f6'), tag('t2', 'Mandiri', '#10b981')]

// covers: INV-TAGS-05
describe('aggregateTagBreakdown', () => {
  it('returns [] for no rows', () => {
    expect(aggregateTagBreakdown([], tags, 'Untagged')).toEqual([])
  })

  it('sums holdings (asset+receivable+investment) and keeps liabilities separate', () => {
    const rows = [
      row('t1', 'asset', 'IDR', '100'),
      row('t1', 'investment', 'IDR', '50'),
      row('t1', 'receivable', 'IDR', '10'),
      row('t1', 'liability', 'IDR', '40'),
    ]
    const [bd] = aggregateTagBreakdown(rows, tags, 'Untagged')
    expect(bd.currency).toBe('IDR')
    expect(bd.cells).toHaveLength(1)
    const cell = bd.cells[0]
    expect(cell.tagId).toBe('t1')
    expect(cell.name).toBe('BCA')
    expect(cell.holdings).toBe(160)
    expect(cell.liabilities).toBe(40)
    expect(cell.net).toBe(120)
    expect(bd.totalHoldings).toBe(160)
    expect(bd.totalLiabilities).toBe(40)
  })

  it('puts null tag_id into an Untagged bucket with the muted colour', () => {
    const rows = [row(null, 'asset', 'IDR', '70')]
    const [bd] = aggregateTagBreakdown(rows, tags, 'Untagged')
    expect(bd.cells[0]).toMatchObject({
      tagId: null,
      name: 'Untagged',
      color: UNTAGGED_COLOR,
      holdings: 70,
    })
  })

  it('sorts tags by holdings desc with Untagged always last', () => {
    const rows = [
      row(null, 'asset', 'IDR', '999'), // untagged biggest, must still sink
      row('t1', 'asset', 'IDR', '30'),
      row('t2', 'asset', 'IDR', '80'),
    ]
    const [bd] = aggregateTagBreakdown(rows, tags, 'Untagged')
    expect(bd.cells.map((c) => c.tagId)).toEqual(['t2', 't1', null])
  })

  it('separates currencies into their own breakdowns, ordered by code', () => {
    const rows = [
      row('t1', 'asset', 'USD', '5'),
      row('t1', 'asset', 'IDR', '5'),
    ]
    const out = aggregateTagBreakdown(rows, tags, 'Untagged')
    expect(out.map((b) => b.currency)).toEqual(['IDR', 'USD'])
  })

  it('ignores non-numeric totals defensively', () => {
    const rows = [row('t1', 'asset', 'IDR', 'not-a-number')]
    expect(aggregateTagBreakdown(rows, tags, 'Untagged')).toEqual([])
  })
})
