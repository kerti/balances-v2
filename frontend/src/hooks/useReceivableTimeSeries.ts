// One household-scoped fetch of every receivable's monthly value series for
// the Receivables list total-over-time chart (epic #204) — the value-only twin
// of useAssetTimeSeries. Receivables carry no cost basis, so the backend
// (`repo/receivable_time_series.go`) returns value series only.

import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

export type ReceivableTimeSeriesItem = {
  receivable_id: string
  value_series: Array<{ year_month: string; amount: string }>
}

export type ReceivablePositionSeries = {
  snapshots: Array<{ year_month: string; amount: string }>
}

export function useReceivableTimeSeries(): {
  byId: Map<string, ReceivablePositionSeries>
  isLoading: boolean
  hasError: boolean
} {
  const query = useQuery({
    queryKey: ['receivable-time-series'],
    queryFn: () =>
      api<ReceivableTimeSeriesItem[]>('/api/receivables/time-series'),
  })

  const byId = useMemo(() => {
    const m = new Map<string, ReceivablePositionSeries>()
    for (const it of query.data ?? []) {
      m.set(it.receivable_id, { snapshots: it.value_series })
    }
    return m
  }, [query.data])

  return { byId, isLoading: query.isPending, hasError: query.isError }
}
