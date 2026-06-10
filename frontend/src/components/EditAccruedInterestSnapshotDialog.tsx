import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { UseMutationResult } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { errorMessage } from '@/lib/errorMessage'
import { formatCurrency } from '@/lib/format'
import { monthStartDate, monthEndDateCapped } from '@/lib/dateLimits'
import type { UpdateInvestmentSnapshotPayload } from '@/hooks/useInvestmentSnapshots'

type AccruedInterestSnapshotLike = {
  id: string
  year_month: string
  amount: string
  currency: string
  accrued_interest: string | null
  as_of_date: string | null
  description: string | null
}

export type UpdateAccruedInterestSnapshotMutationVariables = {
  snapshotId: string
  payload: UpdateInvestmentSnapshotPayload
}

type Props<TResult> = {
  open: boolean
  onOpenChange: (open: boolean) => void
  snapshot: AccruedInterestSnapshotLike
  mutation: UseMutationResult<
    TResult,
    unknown,
    UpdateAccruedInterestSnapshotMutationVariables
  >
}

function derivePrincipal(amount: string, accrued: string): string | null {
  const a = Number(amount)
  const i = Number(accrued)
  if (!amount || !accrued || Number.isNaN(a) || Number.isNaN(i)) {
    return null
  }
  return (a - i).toString()
}

export function EditAccruedInterestSnapshotDialog<TResult>({
  open,
  onOpenChange,
  snapshot,
  mutation,
}: Props<TResult>) {
  const { t } = useTranslation(['investments', 'common'])
  const [form, setForm] = useState({
    amount: snapshot.amount,
    accrued_interest: snapshot.accrued_interest ?? '',
    as_of_date: snapshot.as_of_date ? snapshot.as_of_date.slice(0, 10) : '',
    description: snapshot.description ?? '',
  })

  const derivedPrincipal = derivePrincipal(form.amount, form.accrued_interest)

  function submit(e: React.FormEvent) {
    e.preventDefault()
    mutation.mutate(
      {
        snapshotId: snapshot.id,
        payload: {
          amount: form.amount,
          currency: snapshot.currency,
          quantity: null,
          price_per_unit: null,
          accrued_interest: form.accrued_interest,
          as_of_date: form.as_of_date || null,
          description: form.description || null,
        },
      },
      { onSuccess: () => onOpenChange(false) },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('investments:accruedInterestSnapshot.editTitle')}</DialogTitle>
          <DialogDescription>
            {t('investments:accruedInterestSnapshot.editDescription')}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="edit_ai_amount">
                {t('investments:accruedInterestSnapshot.totalValueLabel', {
                  currency: snapshot.currency,
                })}
              </Label>
              <Input
                id="edit_ai_amount"
                required
                inputMode="decimal"
                value={form.amount}
                onChange={(e) => setForm({ ...form, amount: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit_ai_accrued">
                {t('investments:accruedInterestSnapshot.accruedLabel', {
                  currency: snapshot.currency,
                })}
              </Label>
              <Input
                id="edit_ai_accrued"
                required
                inputMode="decimal"
                value={form.accrued_interest}
                onChange={(e) =>
                  setForm({ ...form, accrued_interest: e.target.value })
                }
              />
            </div>
          </div>

          <div className="rounded-md bg-muted px-3 py-2 text-sm">
            <span className="text-muted-foreground">
              {t('investments:accruedInterestSnapshot.ofWhichPrincipalLabel')}
            </span>{' '}
            <span className="font-medium">
              {derivedPrincipal !== null
                ? formatCurrency(derivedPrincipal, snapshot.currency)
                : '—'}
            </span>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="edit_ai_as_of_date">
              {t('common:fields.statementDate')}
            </Label>
            <Input
              id="edit_ai_as_of_date"
              type="date"
              min={monthStartDate(snapshot.year_month)}
              max={monthEndDateCapped(snapshot.year_month)}
              value={form.as_of_date}
              onChange={(e) =>
                setForm({ ...form, as_of_date: e.target.value })
              }
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="edit_ai_description">
              {t('common:fields.description')}
            </Label>
            <Input
              id="edit_ai_description"
              value={form.description}
              onChange={(e) =>
                setForm({ ...form, description: e.target.value })
              }
            />
          </div>

          {mutation.isError && (
            <p className="text-sm text-destructive">
              {errorMessage(mutation.error)}
            </p>
          )}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
            >
              {t('common:cancel')}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending
                ? t('common:actions.saving')
                : t('common:actions.saveChanges')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
