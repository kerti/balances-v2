import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { MoreHorizontal } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { TableCell, TableRow } from '@/components/ui/table'
import { EditBondDialog } from '@/components/EditBondDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { useDeleteBond } from '@/hooks/useInvestments'
import { formatCurrency, formatYearMonth } from '@/lib/format'
import { StatusBadge } from '@/components/StatusBadge'
import { isActiveStatus } from '@/lib/lifecycle'
import { cn } from '@/lib/utils'
import { RiskProfileBadge } from '@/components/RiskProfileBadge'
import { maturityClass, maturityInfo } from '@/lib/maturity'
import { TransactionActivityCell } from '@/components/TransactionActivityCell'
import type { BondListItem, CouponFrequency } from '@/api/types'

type Props = {
  item: BondListItem
  onSelect: (id: string) => void
}

export function BondListRow({ item, onSelect }: Props) {
  const { t } = useTranslation(['investments', 'common'])
  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const deleteMutation = useDeleteBond()

  const terminated = !isActiveStatus(item.investment.status)

  function handleConfirmDelete() {
    deleteMutation.mutate(item.investment.id, {
      onSuccess: () => setDeleteOpen(false),
    })
  }

  const mInfo = maturityInfo(item.details.maturity_date)
  const couponPct = Number(item.details.coupon_rate).toFixed(2)
  const bondType = t(
    item.details.bond_type === 'govt_primary'
      ? 'investments:bond.bondType.govt_primary_short'
      : 'investments:bond.bondType.secondary_market_short',
  )
  const frequencyShortKey: Record<CouponFrequency, string> = {
    monthly: 'investments:bond.couponFrequency.monthly_short',
    quarterly: 'investments:bond.couponFrequency.quarterly_short',
    semi_annual: 'investments:bond.couponFrequency.semi_annual_short',
    annual: 'investments:bond.couponFrequency.annual_short',
  }
  const frequency = t(frequencyShortKey[item.details.coupon_frequency])

  return (
    <>
      <TableRow
        className={cn('cursor-pointer', terminated && 'text-muted-foreground')}
        onClick={() => onSelect(item.investment.id)}
      >
        <TableCell>
          <div className="flex items-center gap-2">
            <div className={cn('font-medium', terminated && 'font-normal')}>
              {item.investment.display_name}
            </div>
            <RiskProfileBadge profile={item.investment.risk_profile} compact />
          </div>
          {item.investment.description && (
            <div className="text-xs text-muted-foreground">
              {item.investment.description}
            </div>
          )}
        </TableCell>
        <TableCell>
          {item.details.series_code ? (
            <div className="font-mono text-sm">{item.details.series_code}</div>
          ) : (
            <div className="text-sm text-muted-foreground">—</div>
          )}
          <div className="text-xs text-muted-foreground">
            {t('investments:bond.rowMeta', {
              type: bondType,
              issuer: item.details.issuer,
              rate: couponPct,
              frequency,
            })}
          </div>
          {!terminated && (
            <div className={`text-xs ${maturityClass(mInfo.state)}`}>
              {mInfo.label}
            </div>
          )}
        </TableCell>
        <TableCell>
          <StatusBadge group="investments" status={item.investment.status} />
        </TableCell>
        <TableCell className="text-right tabular-nums">
          {item.latest_snapshot ? (
            <>
              <div>
                {formatCurrency(
                  item.latest_snapshot.amount,
                  item.latest_snapshot.currency,
                )}
              </div>
              <div className="text-xs text-muted-foreground">
                {formatYearMonth(item.latest_snapshot.year_month)}
              </div>
            </>
          ) : (
            <span className="text-muted-foreground">—</span>
          )}
        </TableCell>
        <TransactionActivityCell
          count={item.transaction_count}
          lastDate={item.last_transaction_date}
        />
        <TableCell className="text-right">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t('investments:bond.rowActions')}
                onClick={(e) => e.stopPropagation()}
              >
                <MoreHorizontal className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              align="end"
              onClick={(e) => e.stopPropagation()}
            >
              <DropdownMenuItem onClick={() => setEditOpen(true)}>
                {t('common:actions.edit')}
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => setDeleteOpen(true)}
                variant="destructive"
              >
                {t('common:delete')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </TableCell>
      </TableRow>

      <EditBondDialog
        key={item.investment.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        bond={item}
      />

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('investments:bond.deleteTitle')}
        description={t('investments:bond.deleteRowDescription', {
          name: item.investment.display_name,
        })}
        confirmLabel={t('common:delete')}
        destructive
        pending={deleteMutation.isPending}
        onConfirm={handleConfirmDelete}
      />
    </>
  )
}
