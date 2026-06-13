import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Me } from '@/hooks/useSession'

// Updates the current user's own profile. Fields are independent: pass
// `nickname` to set/clear the compact owner label (null/"" clears), pass
// `locale` to switch the UI language (BCP47, e.g. 'en-GB' / 'id-ID'), pass
// `theme` to switch the UI theme ('light' / 'dark'), pass `carryover_date_mode`
// to change how the carryover dialog seeds its as-of date (issue #105). Any
// combination in one payload is fine; the backend updates each independently.
// Refreshes session (carries nickname for the "(you)" label, locale + theme for
// boot detection) and household-members (picker labels resolve via nickname).
export type UpdateMePayload = {
  nickname?: string | null
  locale?: string
  theme?: string
  carryover_date_mode?: string
}

export function useUpdateMe() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (p: UpdateMePayload) =>
      api<Me>('/api/me', { method: 'PATCH', body: JSON.stringify(p) }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['session'] })
      qc.invalidateQueries({ queryKey: ['household-members'] })
    },
  })
}
