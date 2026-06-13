import { useState } from 'react'
import { CopyPlus, Plus } from 'lucide-react'
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
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { errorMessage } from '@/lib/errorMessage'
import { formatCurrency } from '@/lib/format'
import { thisYearMonth, todayDate, carryoverSeedDate } from '@/lib/dateLimits'
import type { CarryoverDateMode } from '@/lib/dateLimits'
import { useSession } from '@/hooks/useSession'
import type { CreateInvestmentSnapshotPayload } from '@/hooks/useInvestmentSnapshots'

type Props<TResult> = {
  currency: string
  mutation: UseMutationResult<
    TResult,
    unknown,
    CreateInvestmentSnapshotPayload
  >
  // Latest snapshot's total value + accrued + period, when one exists. Drives
  // the "Copy carryover" helper (issue #60); lastSnapshotMonth (the latest
  // snapshot's year_month) anchors the end_of_month_after_last_snapshot date
  // mode (issue #105). Null hides the helper.
  carryover?: {
    amount: string
    accrued_interest: string | null
    lastSnapshotMonth: string
  } | null
}

function emptyForm() {
  // accrued defaults to 0: covers the common Indonesian-govt-primary case
  // where coupons pay out to the bank account directly (no in-instrument
  // accrual). Secondary-market bond + TimeDeposit users override.
  return {
    year_month: thisYearMonth(),
    amount: '',
    accrued_interest: '0',
    as_of_date: '',
    description: '',
  }
}

// `amount` is the dirty total value (already includes accrued); the user
// types it as it appears on the statement. The derived "of which principal"
// line below = amount − accrued. The backend's validateInvestmentSnapshotShape
// re-checks both fields are present for bond/time_deposit subtypes.
function derivePrincipal(amount: string, accrued: string): string | null {
  const a = Number(amount)
  const i = Number(accrued)
  if (!amount || !accrued || Number.isNaN(a) || Number.isNaN(i)) {
    return null
  }
  return (a - i).toString()
}

export function CreateAccruedInterestSnapshotDialog<TResult>({
  currency,
  mutation,
  carryover,
}: Props<TResult>) {
  const { t } = useTranslation(['investments', 'common'])
  const { data: me } = useSession()
  const [open, setOpen] = useState(false)
  const [form, setForm] = useState(emptyForm)

  const derivedPrincipal = derivePrincipal(form.amount, form.accrued_interest)

  // Seed the form from the last snapshot and open the dialog. Month resets to
  // the current month; the statement date is seeded per the user's
  // carryover_date_mode preference (issue #105, default 'today').
  function startCarryover() {
    if (!carryover) return
    const mode = (me?.carryover_date_mode ?? 'today') as CarryoverDateMode
    setForm({
      year_month: thisYearMonth(),
      amount: carryover.amount,
      accrued_interest: carryover.accrued_interest ?? '0',
      as_of_date: carryoverSeedDate(mode, carryover.lastSnapshotMonth),
      description: '',
    })
    setOpen(true)
  }

  function close() {
    setOpen(false)
    setForm(emptyForm())
    mutation.reset()
  }

  function submit(e: React.FormEvent) {
    e.preventDefault()
    mutation.mutate(
      {
        year_month: form.year_month,
        amount: form.amount,
        currency,
        quantity: null,
        price_per_unit: null,
        accrued_interest: form.accrued_interest,
        as_of_date: form.as_of_date || null,
        description: form.description || null,
      },
      { onSuccess: close },
    )
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      {carryover && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={startCarryover}
          data-testid="snapshot-carryover"
        >
          <CopyPlus className="mr-1 size-4" />
          {t('investments:accruedInterestSnapshot.carryoverTrigger')}
        </Button>
      )}
      <DialogTrigger asChild>
        <Button size="sm">
          <Plus className="mr-1 size-4" />
          {t('investments:accruedInterestSnapshot.trigger')}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {t('investments:accruedInterestSnapshot.createTitle')}
          </DialogTitle>
          <DialogDescription>
            {t('investments:accruedInterestSnapshot.createDescription', { currency })}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="ai_year_month">{t('common:fields.month')}</Label>
              <Input
                id="ai_year_month"
                type="month"
                required
                max={thisYearMonth()}
                value={form.year_month}
                onChange={(e) =>
                  setForm({ ...form, year_month: e.target.value })
                }
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="ai_as_of_date">
                {t('common:fields.statementDate')}
              </Label>
              <Input
                id="ai_as_of_date"
                type="date"
                max={todayDate()}
                value={form.as_of_date}
                onChange={(e) =>
                  setForm({ ...form, as_of_date: e.target.value })
                }
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="ai_amount">
                {t('investments:accruedInterestSnapshot.totalValueLabel', { currency })}
              </Label>
              <Input
                id="ai_amount"
                required
                inputMode="decimal"
                value={form.amount}
                onChange={(e) => setForm({ ...form, amount: e.target.value })}
                placeholder={t('investments:accruedInterestSnapshot.totalValuePlaceholder')}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="ai_accrued">
                {t('investments:accruedInterestSnapshot.accruedLabel', { currency })}
              </Label>
              <Input
                id="ai_accrued"
                required
                inputMode="decimal"
                value={form.accrued_interest}
                onChange={(e) =>
                  setForm({ ...form, accrued_interest: e.target.value })
                }
                placeholder={t('investments:accruedInterestSnapshot.accruedPlaceholder')}
              />
            </div>
          </div>

          <div className="rounded-md bg-muted px-3 py-2 text-sm">
            <span className="text-muted-foreground">
              {t('investments:accruedInterestSnapshot.ofWhichPrincipalLabel')}
            </span>{' '}
            <span className="font-medium">
              {derivedPrincipal !== null
                ? formatCurrency(derivedPrincipal, currency)
                : '—'}
            </span>
          </div>

          <p className="text-xs text-muted-foreground">
            {t('investments:accruedInterestSnapshot.accruedHint')}
          </p>

          <div className="grid gap-2">
            <Label htmlFor="ai_description">
              {t('common:fields.description')}
            </Label>
            <Input
              id="ai_description"
              value={form.description}
              onChange={(e) =>
                setForm({ ...form, description: e.target.value })
              }
              placeholder={t('investments:accruedInterestSnapshot.descriptionPlaceholder')}
            />
          </div>

          {mutation.isError && (
            <p className="text-sm text-destructive">
              {errorMessage(mutation.error)}
            </p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t('common:cancel')}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending
                ? t('common:actions.saving')
                : t('investments:accruedInterestSnapshot.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
