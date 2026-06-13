import { useQuery } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'

export type Me = {
  id: string
  household_id: string
  display_name: string
  nickname: string | null
  email: string
  picture_url: string | null
  locale: string
  theme: string
  carryover_date_mode: string
  time_zone: string
  reporting_currency: string
  multi_currency_enabled: boolean
}

// useSession returns:
//   isPending=true        — initial fetch in flight
//   data===null           — not signed in (401 from /api/me)
//   data=<Me>             — signed in
//   error                 — a real error (network, 5xx, etc.)
export function useSession() {
  return useQuery<Me | null>({
    queryKey: ['session'],
    queryFn: async () => {
      try {
        return await api<Me>('/api/me')
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          return null
        }
        throw err
      }
    },
    staleTime: 60_000,
    refetchOnWindowFocus: true,
  })
}
