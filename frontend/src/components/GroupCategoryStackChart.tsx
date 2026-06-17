// Generic 100%-stacked category-share chart (epic #204) — the group-Home
// twin of CategoryStackChart, which is hardwired to the 5 investment
// categories. This one takes its categories (key + label + color) as a prop,
// so AssetsHome (bankAccount/property/vehicle) and LiabilitiesHome
// (personal/institutional) reuse one impl. Lazy boundary so recharts lands in
// a separate chunk, matching the investment chart.

import { lazy, Suspense } from 'react'
import type { GroupCategoryTimePoint } from '@/lib/groupHomeAggregates'

const GroupCategoryStackChartImpl = lazy(
  () => import('./GroupCategoryStackChartImpl'),
)

export type GroupStackCategory = {
  key: string
  label: string
  color: string
}

type Props = {
  series: GroupCategoryTimePoint[]
  categories: GroupStackCategory[]
}

export function GroupCategoryStackChart({ series, categories }: Props) {
  if (series.length < 2) return null
  return (
    <Suspense fallback={<div className="h-64 w-full" />}>
      <GroupCategoryStackChartImpl series={series} categories={categories} />
    </Suspense>
  )
}
