import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { OwnershipField } from "@/components/common/OwnershipField";
import { Select } from "@/components/ui/select";
import { useCreateIncome } from "@/hooks/useIncome";
import { useSession } from "@/hooks/useSession";
import { errorMessage } from "@/lib/errorMessage";
import type { IncomeCategory, Regularity } from "@/api/types";

// todayISO returns YYYY-MM-DD in the local timezone. toISOString() would shift
// users east of UTC into yesterday for the first hours of their day.
function todayISO(): string {
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

type FormState = {
  date: string;
  amount: string;
  currency: string;
  category: IncomeCategory | "";
  description: string;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  regularity: Regularity;
  // True once the user has hand-picked a regularity. Guards the smart default
  // (see categoryDefaultRegularity) from clobbering a deliberate choice when
  // the category is changed afterwards.
  regularityTouched: boolean;
};

// The category answers "can you count on this regularly?" for the common case,
// so we pre-select the matching regularity when a category is chosen. The user
// can always flip it; the genuine edge (lumpy-but-relied-upon gig income) is
// where they will.
const categoryDefaultRegularity: Record<IncomeCategory, Regularity> = {
  salary: "routine",
  business_income: "routine",
  rental_income: "routine",
  pension: "routine",
  interest: "routine",
  gift: "incidental",
  tax_refund: "incidental",
  insurance_payout: "incidental",
  other: "routine",
};

export type DuplicateSeed = {
  amount: string;
  currency: string;
  category: IncomeCategory;
  description: string | null;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  regularity: Regularity;
};

type Props = {
  /** Controlled mode. If provided, parent owns open state. */
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  /** Pre-fill from an existing row (Duplicate flow). Parent must remount the
   *  dialog (key={seedId}) when seed changes — initial state comes from the
   *  useState initializer, not a useEffect. */
  seed?: DuplicateSeed;
  /** Suppress the default "+ New income" trigger button. */
  hideTrigger?: boolean;
  /** Use the compact "New" trigger label (mobile toolbar) instead of the full
   *  "New income". The `+` icon stays so it still reads as an action, not a
   *  filter pill. */
  compactTrigger?: boolean;
};

function initialForm(seed?: DuplicateSeed): FormState {
  if (!seed) {
    return {
      date: todayISO(),
      amount: "",
      currency: "IDR",
      category: "",
      description: "",
      ownership_type: "sole",
      sole_owner_user_id: null,
      // Default routine: salary-dominant case (M4.5 grilling lineage).
      regularity: "routine",
      regularityTouched: false,
    };
  }
  return {
    date: todayISO(),
    amount: seed.amount,
    currency: seed.currency,
    category: seed.category,
    description: seed.description ?? "",
    ownership_type: seed.ownership_type,
    sole_owner_user_id: seed.sole_owner_user_id,
    regularity: seed.regularity,
    // A duplicated row's regularity was a deliberate choice — treat it as
    // touched so changing the category here won't overwrite it.
    regularityTouched: true,
  };
}

export function CreateIncomeDialog({
  open: controlledOpen,
  onOpenChange,
  seed,
  hideTrigger = false,
  compactTrigger = false,
}: Props = {}) {
  const { t } = useTranslation(["income", "common"]);
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const isControlled = controlledOpen !== undefined;
  const open = isControlled ? controlledOpen : uncontrolledOpen;

  const [form, setForm] = useState<FormState>(() => initialForm(seed));
  const { data: user } = useSession();
  const mutation = useCreateIncome();

  // Default the sole-owner picker to the current user the first time we know
  // who they are. If a seed pre-fills sole_owner_user_id, that takes priority.
  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function close() {
    if (isControlled) {
      onOpenChange?.(false);
    } else {
      setUncontrolledOpen(false);
      setForm(initialForm(seed));
    }
    mutation.reset();
  }

  function openDialog() {
    if (isControlled) {
      onOpenChange?.(true);
    } else {
      setUncontrolledOpen(true);
    }
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!user) return;
    if (!form.category) return;
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
      { onSuccess: close },
    );
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? openDialog() : close())}>
      {!hideTrigger && (
        <DialogTrigger asChild>
          <Button>
            <Plus className="mr-1 size-4" />
            {t(compactTrigger ? "income:createTriggerShort" : "income:createTrigger")}
          </Button>
        </DialogTrigger>
      )}
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{seed ? t("income:duplicateTitle") : t("income:createTitle")}</DialogTitle>
          <DialogDescription>
            {seed ? t("income:duplicateDescription") : t("income:createDescription")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          {/* Date and category used to share a two-column row, which truncated
              every longer category label (#572). Not a phone-only squeeze, so
              not a `md:` divergence: `DialogContent` is `sm:max-w-sm`, so the
              column is ~170px at *every* width, and the longest options don't
              fit it in either locale ("Insurance payout", "Pendapatan pensiun")
              — a household member picking income they can't read the name of is
              exactly the bar INV-PRESENTATION-08 sets. The chevron's `pr-8`
              made it worse than it looked on the native control it replaced. */}
          <div className="grid gap-2">
            <Label htmlFor="income_date">{t("income:fields.date")}</Label>
            <Input
              id="income_date"
              type="date"
              required
              max="9999-12-31"
              value={form.date}
              onChange={(e) => setForm({ ...form, date: e.target.value })}
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="income_category">{t("income:fields.category")}</Label>
            <Select
              id="income_category"
              required
              value={form.category}
              onChange={(e) => {
                const category = e.target.value as IncomeCategory;
                setForm({
                  ...form,
                  category,
                  // Smart default: pre-select regularity from the category
                  // unless the user has already hand-picked one.
                  regularity: form.regularityTouched
                    ? form.regularity
                    : categoryDefaultRegularity[category],
                });
              }}
            >
              <option value="" disabled>
                {t("income:categoryOptions.placeholder")}
              </option>
              <option value="salary">{t("income:categoryOptions.salary")}</option>
              <option value="business_income">{t("income:categoryOptions.business_income")}</option>
              <option value="rental_income">{t("income:categoryOptions.rental_income")}</option>
              <option value="pension">{t("income:categoryOptions.pension")}</option>
              <option value="interest">{t("income:categoryOptions.interest")}</option>
              <option value="gift">{t("income:categoryOptions.gift")}</option>
              <option value="tax_refund">{t("income:categoryOptions.tax_refund")}</option>
              <option value="insurance_payout">
                {t("income:categoryOptions.insurance_payout")}
              </option>
              <option value="other">{t("income:categoryOptions.other")}</option>
            </Select>
          </div>

          <div className="grid grid-cols-[1fr_120px] gap-3">
            <div className="grid gap-2">
              <Label htmlFor="income_amount">{t("income:fields.amount")}</Label>
              <Input
                id="income_amount"
                required
                inputMode="decimal"
                value={form.amount}
                onChange={(e) => setForm({ ...form, amount: e.target.value })}
                placeholder={t("income:placeholders.amount")}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="income_currency">{t("income:fields.currency")}</Label>
              <Input
                id="income_currency"
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
            <Label htmlFor="income_description">{t("income:fields.description")}</Label>
            <Input
              id="income_description"
              value={form.description}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
              placeholder={t("income:placeholders.description")}
            />
          </div>

          <div className="grid gap-2">
            <Label>{t("income:regularity.question")}</Label>
            <div className="grid gap-2 text-sm">
              <label className="flex items-start gap-2">
                <input
                  type="radio"
                  name="regularity"
                  value="routine"
                  className="mt-0.5"
                  checked={form.regularity === "routine"}
                  onChange={() =>
                    setForm({ ...form, regularity: "routine", regularityTouched: true })
                  }
                />
                <span>
                  <span className="font-medium">{t("income:regularity.routineOption")}</span>
                  <span className="block text-xs text-muted-foreground">
                    {t("income:regularity.routineHint")}
                  </span>
                </span>
              </label>
              <label className="flex items-start gap-2">
                <input
                  type="radio"
                  name="regularity"
                  value="incidental"
                  className="mt-0.5"
                  checked={form.regularity === "incidental"}
                  onChange={() =>
                    setForm({ ...form, regularity: "incidental", regularityTouched: true })
                  }
                />
                <span>
                  <span className="font-medium">{t("income:regularity.incidentalOption")}</span>
                  <span className="block text-xs text-muted-foreground">
                    {t("income:regularity.incidentalHint")}
                  </span>
                </span>
              </label>
            </div>
          </div>

          <OwnershipField
            idPrefix="income_create"
            value={form.ownership_type}
            onChange={(v) => setForm({ ...form, ownership_type: v })}
            soleOwnerID={effectiveSoleOwnerID}
            onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
          />

          {mutation.error && (
            <p className="text-sm text-destructive">{errorMessage(mutation.error)}</p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t("income:submit.saving") : t("income:submit.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
