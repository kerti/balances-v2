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
import { EditGoldDialog } from '@/components/EditGoldDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { useDeleteGold } from '@/hooks/useInvestments'
import { formatCurrency, formatYearMonth } from '@/lib/format'
import { StatusBadge } from '@/components/StatusBadge'
import { isActiveStatus } from '@/lib/lifecycle'
import { cn } from '@/lib/utils'
import { RiskProfileBadge } from '@/components/RiskProfileBadge'
import { formatGoldPurity } from '@/lib/gold'
import { TransactionActivityCell } from '@/components/TransactionActivityCell'
import type { GoldListItem } from '@/api/types'

type Props = {
  item: GoldListItem
  onSelect: (id: string) => void
}

export function GoldListRow({ item, onSelect }: Props) {
  const { t } = useTranslation(['investments', 'common'])
  const [editOpen, setEditOpen] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const deleteMutation = useDeleteGold()

  const terminated = !isActiveStatus(item.investment.status)

  function handleConfirmDelete() {
    deleteMutation.mutate(item.investment.id, {
      onSuccess: () => setDeleteOpen(false),
    })
  }

  const formLabel = t(`investments:gold.goldForms.${item.details.form}`)

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
          <div>{formLabel}</div>
          <div className="text-xs text-muted-foreground">
            {formatGoldPurity(item.details.purity)}
          </div>
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
                aria-label={t('investments:gold.rowActions')}
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

      <EditGoldDialog
        key={item.investment.id}
        open={editOpen}
        onOpenChange={setEditOpen}
        gold={item}
      />

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('investments:gold.deleteTitle')}
        description={t('investments:gold.deleteRowDescription', {
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
