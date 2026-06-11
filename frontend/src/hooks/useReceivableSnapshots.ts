import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { ReceivableSnapshot } from '@/api/types'
import {
  postSnapshotImport,
  snapshotImportTemplateUrl,
  type ImportArgs,
} from './snapshotImport'

// Receivable snapshots live under /api/receivables/{id}/snapshots — per-group
// per ADR-0022.

export type CreateReceivableSnapshotPayload = {
  year_month: string
  amount: string
  currency: string
  as_of_date: string | null
  description: string | null
}

export type UpdateReceivableSnapshotPayload = {
  amount: string
  currency: string
  as_of_date: string | null
  description: string | null
}

export function useReceivableSnapshots(receivableId: string | null) {
  return useQuery({
    queryKey: ['receivable-snapshots', receivableId],
    queryFn: () =>
      api<ReceivableSnapshot[]>(`/api/receivables/${receivableId}/snapshots`),
    enabled: !!receivableId,
  })
}

export function useCreateReceivableSnapshot(receivableId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload: CreateReceivableSnapshotPayload) =>
      api<ReceivableSnapshot>(`/api/receivables/${receivableId}/snapshots`, {
        method: 'POST',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['receivable-snapshots', receivableId] })
      qc.invalidateQueries({ queryKey: ['receivables'] })
    },
  })
}

export function useUpdateReceivableSnapshot(receivableId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (args: {
      snapshotId: string
      payload: UpdateReceivableSnapshotPayload
    }) =>
      api<ReceivableSnapshot>(
        `/api/receivables/${receivableId}/snapshots/${args.snapshotId}`,
        {
          method: 'PATCH',
          body: JSON.stringify(args.payload),
        },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['receivable-snapshots', receivableId] })
      qc.invalidateQueries({ queryKey: ['receivables'] })
    },
  })
}

export function useDeleteReceivableSnapshot(receivableId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (snapshotId: string) =>
      api(`/api/receivables/${receivableId}/snapshots/${snapshotId}`, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['receivable-snapshots', receivableId] })
      qc.invalidateQueries({ queryKey: ['receivables'] })
    },
  })
}

// ----- bulk snapshot import (xlsx template) -------------------------------

export function receivableImportTemplateUrl(receivableId: string): string {
  return snapshotImportTemplateUrl(`/api/receivables/${receivableId}/snapshots`)
}

// receivableExportUrl is the plain-GET download for a receivable's full position
// workbook (Detail + Snapshots). The session cookie rides along same-origin, so
// a bare anchor is enough.
export function receivableExportUrl(receivableId: string): string {
  return `/api/receivables/${receivableId}/export`
}

export function useImportReceivableSnapshots(receivableId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (args: ImportArgs) =>
      postSnapshotImport(
        `/api/receivables/${receivableId}/snapshots`,
        args.file,
        args.mode,
      ),
    onSuccess: (result) => {
      // Only a real write should refresh the caches; a preview changed nothing.
      if (result.committed) {
        qc.invalidateQueries({ queryKey: ['receivable-snapshots', receivableId] })
        qc.invalidateQueries({ queryKey: ['receivables'] })
      }
    },
  })
}
