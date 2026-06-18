// 100%-stacked category share chart (issue #14 slice 14d). Lazy
// boundary so the recharts + shadcn-chart code lands in a separate
// chunk alongside the other home charts.

import { Suspense } from 'react'
import { lazyWithReload } from '@/lib/lazyWithReload'
import type { CategoryTimePoint } from '@/lib/homeAggregates'

const CategoryStackChartImpl = lazyWithReload(
  () => import('./CategoryStackChartImpl'),
)

type Props = {
  series: CategoryTimePoint[]
}

export function CategoryStackChart({ series }: Props) {
  if (series.length < 2) return null
  return (
    <Suspense fallback={<div className="h-64 w-full" />}>
      <CategoryStackChartImpl series={series} />
    </Suspense>
  )
}
