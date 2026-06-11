import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { postCreateImport, type CreateImportArgs } from '@/hooks/snapshotImport'
import type { Receivable, ReceivableListItem } from '@/api/types'

export type CreateReceivablePayload = {
  display_name: string
  description: string | null
  ownership_type: 'sole' | 'joint'
  sole_owner_user_id: string | null
  native_currency: string
  counterparty_name: string
  due_date: string | null
}

export type UpdateReceivablePayload = {
  display_name: string
  description: string | null
  ownership_type: 'sole' | 'joint'
  sole_owner_user_id: string | null
  counterparty_name: string
  due_date: string | null
}

export function useReceivables() {
  return useQuery({
    queryKey: ['receivables'],
    queryFn: () => api<ReceivableListItem[]>('/api/receivables'),
    staleTime: 10_000,
  })
}

export function useReceivable(id: string | null) {
  return useQuery({
    queryKey: ['receivables', id],
    queryFn: () => api<Receivable>(`/api/receivables/${id}`),
    enabled: !!id,
  })
}

export function useCreateReceivable() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload: CreateReceivablePayload) =>
      api<Receivable>('/api/receivables', {
        method: 'POST',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['receivables'] })
    },
  })
}

export function useUpdateReceivable(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload: UpdateReceivablePayload) =>
      api<Receivable>(`/api/receivables/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['receivables'] })
      qc.invalidateQueries({ queryKey: ['receivables', id] })
    },
  })
}

// useImportCreateReceivable drives the create-from-file dialog on the list
// screen: a preview is a server-side dry-run; a committed create writes a new
// receivable + its snapshots in one transaction, so only that refreshes the list.
export function useImportCreateReceivable() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (args: CreateImportArgs) =>
      postCreateImport('/api/receivables', args.file, args.mode),
    onSuccess: (result) => {
      if (result.committed) {
        qc.invalidateQueries({ queryKey: ['receivables'] })
      }
    },
  })
}

export function useDeleteReceivable() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      api(`/api/receivables/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['receivables'] })
    },
  })
}
