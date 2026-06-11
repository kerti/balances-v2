import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { postCreateImport, type CreateImportArgs } from '@/hooks/snapshotImport'
import type { BankAccount, BankAccountListItem } from '@/api/types'

export type CreateBankAccountPayload = {
  display_name: string
  description: string | null
  ownership_type: 'sole' | 'joint'
  sole_owner_user_id: string | null
  native_currency: string
  bank_name: string
  account_number: string
  account_type: 'savings' | 'current' | 'other'
}

export type UpdateBankAccountPayload = {
  display_name: string
  description: string | null
  ownership_type: 'sole' | 'joint'
  sole_owner_user_id: string | null
  bank_name: string
  account_number: string
  account_type: 'savings' | 'current' | 'other'
}

export function useBankAccounts() {
  return useQuery({
    queryKey: ['bank-accounts'],
    queryFn: () => api<BankAccountListItem[]>('/api/bank-accounts'),
    staleTime: 10_000,
  })
}

export function useBankAccount(id: string | null) {
  return useQuery({
    queryKey: ['bank-accounts', id],
    queryFn: () => api<BankAccount>(`/api/bank-accounts/${id}`),
    enabled: !!id,
  })
}

export function useCreateBankAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload: CreateBankAccountPayload) =>
      api<BankAccount>('/api/bank-accounts', {
        method: 'POST',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['bank-accounts'] })
    },
  })
}

export function useUpdateBankAccount(id: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (payload: UpdateBankAccountPayload) =>
      api<BankAccount>(`/api/bank-accounts/${id}`, {
        method: 'PATCH',
        body: JSON.stringify(payload),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['bank-accounts'] })
      qc.invalidateQueries({ queryKey: ['bank-accounts', id] })
    },
  })
}

// useImportCreateBankAccount drives the create-from-file dialog on the list
// screen: a preview is a server-side dry-run; a committed create writes a new
// bank account + its snapshots in one transaction, so only that refreshes the
// list cache.
export function useImportCreateBankAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (args: CreateImportArgs) =>
      postCreateImport('/api/bank-accounts', args.file, args.mode),
    onSuccess: (result) => {
      if (result.committed) {
        qc.invalidateQueries({ queryKey: ['bank-accounts'] })
      }
    },
  })
}

export function useDeleteBankAccount() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (id: string) =>
      api(`/api/bank-accounts/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['bank-accounts'] })
    },
  })
}

// Snapshot hooks have moved to useAssetSnapshots — they're shared across
// all asset subtypes per ADR-0022. Re-export for backwards compatibility
// with existing imports; new code should import from useAssetSnapshots
// directly.
export {
  useSnapshots,
  useCreateSnapshot,
  useUpdateSnapshot,
  useDeleteSnapshot,
} from './useAssetSnapshots'
export type {
  CreateSnapshotPayload,
  UpdateSnapshotPayload,
} from './useAssetSnapshots'
