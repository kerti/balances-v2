import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUpdateIncome } from "@/hooks/useIncome";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { useSession } from "@/hooks/useSession";
import { errorMessage } from "@/lib/errorMessage";
import type { Income, IncomeCategory } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  income: Income;
};

function toForm(i: Income) {
  return {
    date: i.date.slice(0, 10),
    amount: i.amount,
    currency: i.currency,
    category: i.category,
    description: i.description ?? "",
    ownership_type: i.ownership_type,
    sole_owner_user_id: i.sole_owner_user_id,
    regularity: i.regularity,
  };
}

export function EditIncomeDialog({ open, onOpenChange, income }: Props) {
  const { t } = useTranslation(["income", "common"]);
  const mutation = useUpdateIncome(income.id);
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();
  const [form, setForm] = useState(() => toForm(income));

  // If the original row had no sole_owner (was joint, now flipped to sole),
  // fall back to the current user.
  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!user) return;
    mutation.mutate(
      {
        date: form.date,
        amount: form.amount,
        currency: form.currency,
        category: form.category,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        regularity: form.regularity,
      },
      { onSuccess: () => onOpenChange(false) },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("income:editTitle")}</DialogTitle>
          <DialogDescription>{t("income:editDescription")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="edit_income_date">{t("income:fields.date")}</Label>
              <Input
                id="edit_income_date"
                type="date"
                required
                max="9999-12-31"
                value={form.date}
                onChange={(e) => setForm({ ...form, date: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit_income_category">{t("income:fields.category")}</Label>
              <select
                id="edit_income_category"
                required
                className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                value={form.category}
                onChange={(e) =>
                  setForm({
                    ...form,
                    category: e.target.value as IncomeCategory,
                  })
                }
              >
                <option value="salary">{t("income:categoryOptions.salary")}</option>
                <option value="business_income">
                  {t("income:categoryOptions.business_income")}
                </option>
                <option value="rental_income">{t("income:categoryOptions.rental_income")}</option>
                <option value="gift">{t("income:categoryOptions.gift")}</option>
                <option value="tax_refund">{t("income:categoryOptions.tax_refund")}</option>
                <option value="insurance_payout">
                  {t("income:categoryOptions.insurance_payout")}
                </option>
                <option value="other">{t("income:categoryOptions.other")}</option>
              </select>
            </div>
          </div>

          <div className="grid grid-cols-[1fr_120px] gap-3">
            <div className="grid gap-2">
              <Label htmlFor="edit_income_amount">{t("income:fields.amount")}</Label>
              <Input
                id="edit_income_amount"
                required
                inputMode="decimal"
                value={form.amount}
                onChange={(e) => setForm({ ...form, amount: e.target.value })}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="edit_income_currency">{t("income:fields.currency")}</Label>
              <Input
                id="edit_income_currency"
                required
                value={form.currency}
                onChange={(e) =>
                  setForm({
                    ...form,
                    currency: e.target.value.toUpperCase(),
                  })
                }
                maxLength={3}
              />
            </div>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="edit_income_description">{t("income:fields.description")}</Label>
            <Input
              id="edit_income_description"
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
            />
          </div>

          <div className="grid gap-2">
            <Label>{t("income:regularity.label")}</Label>
            <div className="flex gap-4 text-sm">
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="edit_regularity"
                  value="routine"
                  checked={form.regularity === "routine"}
                  onChange={() => setForm({ ...form, regularity: "routine" })}
                />
                {t("income:regularity.routine")}
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="edit_regularity"
                  value="incidental"
                  checked={form.regularity === "incidental"}
                  onChange={() => setForm({ ...form, regularity: "incidental" })}
                />
                {t("income:regularity.incidental")}
              </label>
            </div>
          </div>

          <div className="grid gap-2">
            <Label>{t("common:fields.ownership")}</Label>
            <div className="flex gap-4 text-sm">
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="edit_ownership_type"
                  value="sole"
                  checked={form.ownership_type === "sole"}
                  onChange={() => setForm({ ...form, ownership_type: "sole" })}
                />
                {t("common:ownership.soleOwner")}
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="edit_ownership_type"
                  value="joint"
                  checked={form.ownership_type === "joint"}
                  onChange={() => setForm({ ...form, ownership_type: "joint" })}
                />
                {t("common:ownership.joint")}
              </label>
            </div>
            {form.ownership_type === "sole" && (
              <select
                aria-label={t("common:ownership.soleOwner")}
                className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                value={effectiveSoleOwnerID ?? ""}
                onChange={(e) => setForm({ ...form, sole_owner_user_id: e.target.value })}
              >
                {(members ?? []).map((m) => (
                  <option key={m.id} value={m.id}>
                    {preferredName(m)}
                    {user && m.id === user.id ? t("common:ownership.youSuffix") : ""}
                  </option>
                ))}
              </select>
            )}
          </div>

          {mutation.error && (
            <p className="text-sm text-destructive">{errorMessage(mutation.error)}</p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t("common:actions.saving") : t("common:actions.saveChanges")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
