import { useState } from 'react'
import { useTranslation } from 'react-i18next'
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
import { useUpdateTimeDeposit } from '@/hooks/useInvestments'
import { useHouseholdMembers } from '@/hooks/useHouseholdMembers'
import { preferredName } from '@/lib/names'
import { useSession } from '@/hooks/useSession'
import { errorMessage } from '@/lib/errorMessage'
import { RiskProfileSelect } from '@/components/RiskProfileSelect'
import type {
  RolloverPolicy,
  TimeDeposit,
  TimeDepositListItem,
} from '@/api/types'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  timeDeposit: TimeDeposit | TimeDepositListItem
}

function toForm(td: TimeDeposit | TimeDepositListItem) {
  const i = td.investment
  const d = td.details
  return {
    display_name: i.display_name,
    description: i.description ?? '',
    ownership_type: i.ownership_type,
    sole_owner_user_id: i.sole_owner_user_id,
    risk_profile: td.investment.risk_profile,
    bank_name: d.bank_name,
    principal: d.principal,
    interest_rate: d.interest_rate,
    term_months: String(d.term_months),
    placement_date: d.placement_date ? d.placement_date.slice(0, 10) : '',
    maturity_date: d.maturity_date ? d.maturity_date.slice(0, 10) : '',
    rollover_policy: d.rollover_policy,
  }
}

export function EditTimeDepositDialog({
  open,
  onOpenChange,
  timeDeposit,
}: Props) {
  const { t } = useTranslation(['investments', 'common'])
  const [form, setForm] = useState(() => toForm(timeDeposit))
  const { data: user } = useSession()
  const { data: members } = useHouseholdMembers()
  const mutation = useUpdateTimeDeposit(timeDeposit.investment.id)

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null

  // Maturity must stay strictly after placement (issue #62) — mirrors the
  // server's ErrInvalidDepositTerm. (A term that strands existing snapshots is
  // caught server-side as OUTSIDE_DEPOSIT_TERM and surfaced via mutation.error.)
  const termInvalid =
    !!form.placement_date &&
    !!form.maturity_date &&
    form.maturity_date <= form.placement_date

  function submit(e: React.FormEvent) {
    e.preventDefault()
    if (termInvalid) return
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id:
          form.ownership_type === 'sole' ? effectiveSoleOwnerID : null,
        risk_profile: form.risk_profile,
        bank_name: form.bank_name,
        principal: form.principal,
        interest_rate: form.interest_rate,
        term_months: Number(form.term_months),
        placement_date: form.placement_date,
        maturity_date: form.maturity_date,
        rollover_policy: form.rollover_policy,
      },
      { onSuccess: () => onOpenChange(false) },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t('investments:timeDeposit.editTitle')}</DialogTitle>
          <DialogDescription>
            {t('investments:timeDeposit.editDescription')}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-4">
          <div className="space-y-3">
            <div className="grid gap-2">
              <Label htmlFor="edit_td_display_name">
                {t('common:fields.displayName')}
              </Label>
              <Input
                id="edit_td_display_name"
                required
                value={form.display_name}
                onChange={(e) =>
                  setForm({ ...form, display_name: e.target.value })
                }
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit_td_description">
                {t('common:fields.description')}
              </Label>
              <Input
                id="edit_td_description"
                value={form.description}
                onChange={(e) =>
                  setForm({ ...form, description: e.target.value })
                }
              />
            </div>
          </div>

          <div className="space-y-3 border-t pt-4">
            <div className="grid gap-2">
              <Label htmlFor="edit_td_bank_name">
                {t('investments:timeDeposit.fields.bankName')}
              </Label>
              <Input
                id="edit_td_bank_name"
                required
                value={form.bank_name}
                onChange={(e) =>
                  setForm({ ...form, bank_name: e.target.value })
                }
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit_td_principal">
                {t('investments:timeDeposit.fields.principal')}
              </Label>
              <Input
                id="edit_td_principal"
                required
                inputMode="decimal"
                value={form.principal}
                onChange={(e) =>
                  setForm({ ...form, principal: e.target.value })
                }
              />
            </div>
          </div>

          <div className="space-y-3 border-t pt-4">
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-2">
                <Label htmlFor="edit_td_interest_rate">
                  {t('investments:timeDeposit.fields.interestRate')}
                </Label>
                <Input
                  id="edit_td_interest_rate"
                  required
                  inputMode="decimal"
                  value={form.interest_rate}
                  onChange={(e) =>
                    setForm({ ...form, interest_rate: e.target.value })
                  }
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="edit_td_term_months">
                  {t('investments:timeDeposit.fields.termMonths')}
                </Label>
                <Input
                  id="edit_td_term_months"
                  required
                  inputMode="numeric"
                  value={form.term_months}
                  onChange={(e) =>
                    setForm({ ...form, term_months: e.target.value })
                  }
                />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-2">
                <Label htmlFor="edit_td_placement_date">
                  {t('investments:timeDeposit.fields.placementDate')}
                </Label>
                <Input
                  id="edit_td_placement_date"
                  required
                  type="date"
                  max="9999-12-31"
                  value={form.placement_date}
                  onChange={(e) =>
                    setForm({ ...form, placement_date: e.target.value })
                  }
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="edit_td_maturity_date">
                  {t('investments:timeDeposit.fields.maturityDate')}
                </Label>
                <Input
                  id="edit_td_maturity_date"
                  required
                  type="date"
                  min={form.placement_date || undefined}
                  max="9999-12-31"
                  value={form.maturity_date}
                  onChange={(e) =>
                    setForm({ ...form, maturity_date: e.target.value })
                  }
                />
              </div>
            </div>
            {termInvalid && (
              <p
                data-testid="edit-td-term-error"
                className="text-sm text-destructive"
              >
                {t('investments:timeDeposit.maturityAfterPlacement')}
              </p>
            )}
          </div>

          <div className="space-y-3 border-t pt-4">
            <div className="grid gap-2">
              <Label htmlFor="edit_td_rollover_policy">
                {t('investments:timeDeposit.fields.rolloverPolicy')}
              </Label>
              <select
                id="edit_td_rollover_policy"
                className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                value={form.rollover_policy}
                onChange={(e) =>
                  setForm({
                    ...form,
                    rollover_policy: e.target.value as RolloverPolicy,
                  })
                }
              >
                <option value="auto_renew_principal">
                  {t('investments:timeDeposit.rolloverPolicy.auto_renew_principal')}
                </option>
                <option value="auto_renew_with_interest">
                  {t('investments:timeDeposit.rolloverPolicy.auto_renew_with_interest')}
                </option>
                <option value="no_rollover">
                  {t('investments:timeDeposit.rolloverPolicy.no_rollover')}
                </option>
              </select>
            </div>
          </div>

          <div className="space-y-3 border-t pt-4">
            <div className="grid gap-2">
              <Label>{t('common:fields.ownership')}</Label>
              <div className="flex gap-4 text-sm">
                <label className="flex items-center gap-2">
                  <input
                    type="radio"
                    name="edit_td_ownership_type"
                    value="joint"
                    checked={form.ownership_type === 'joint'}
                    onChange={() =>
                      setForm({ ...form, ownership_type: 'joint' })
                    }
                  />
                  {t('investments:ownership.joint')}
                </label>
                <label className="flex items-center gap-2">
                  <input
                    type="radio"
                    name="edit_td_ownership_type"
                    value="sole"
                    checked={form.ownership_type === 'sole'}
                    onChange={() =>
                      setForm({ ...form, ownership_type: 'sole' })
                    }
                  />
                  {t('investments:ownership.soleOwner')}
                </label>
              </div>
              {form.ownership_type === 'sole' && (
                <select
                  aria-label={t('investments:ownership.soleOwnerAria')}
                  className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                  value={effectiveSoleOwnerID ?? ''}
                  onChange={(e) =>
                    setForm({ ...form, sole_owner_user_id: e.target.value })
                  }
                >
                  {(members ?? []).map((m) => (
                    <option key={m.id} value={m.id}>
                      {preferredName(m)}
                      {user && m.id === user.id ? t('common:ownership.youSuffix') : ''}
                    </option>
                  ))}
                </select>
              )}
            </div>
          </div>

          <RiskProfileSelect
            idPrefix="td_edit"
            value={form.risk_profile}
            onChange={(v) => setForm({ ...form, risk_profile: v })}
          />

          {mutation.error && (
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
            <Button type="submit" disabled={mutation.isPending || termInvalid}>
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
