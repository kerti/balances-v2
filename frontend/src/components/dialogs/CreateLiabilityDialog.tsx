import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useCreateLiability } from "@/hooks/useLiabilities";
import { useSession } from "@/hooks/useSession";
import { PositionFormDialog } from "@/components/dialogs/PositionFormDialog";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import type { EntryType } from "@/api/types";
import { Select } from "@/components/ui/select";

type Props = {
  // When the dialog opens from inside an inner-tab (Personal / Institutional),
  // the subtype is fixed by the tab — we pre-fill and hide the selector.
  defaultSubtype?: "personal" | "institutional";
};

function emptyForm(defaultSubtype: "personal" | "institutional") {
  return {
    display_name: "",
    description: "",
    subtype: defaultSubtype,
    ownership_type: "joint" as "sole" | "joint",
    entry_type: "acquired" as EntryType,
    sole_owner_user_id: null as string | null,
    native_currency: "IDR",
    counterparty_name: "",
    principal: "",
    interest_rate: "",
    term_months: "",
    start_date: "",
    maturity_date: "",
  };
}

export function CreateLiabilityDialog({ defaultSubtype = "personal" }: Props) {
  const { t } = useTranslation(["liabilities", "common"]);
  const [form, setForm] = useState(emptyForm(defaultSubtype));
  const { data: user } = useSession();
  const mutation = useCreateLiability();

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(close: () => void) {
    if (!user) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        subtype: form.subtype,
        ownership_type: form.ownership_type,
        entry_type: form.entry_type,
        sole_owner_user_id: form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        native_currency: form.native_currency,
        counterparty_name: form.counterparty_name,
        principal: form.principal || null,
        interest_rate: form.interest_rate || null,
        term_months: form.term_months ? Number(form.term_months) : null,
        start_date: form.start_date || null,
        maturity_date: form.maturity_date || null,
      },
      { onSuccess: close },
    );
  }

  return (
    <PositionFormDialog
      trigger={
        <Button>
          <Plus className="mr-1 size-4" />
          {t("liabilities:createTrigger")}
        </Button>
      }
      title={t("liabilities:createTitle")}
      description={t("liabilities:createDescription")}
      submitLabel={t("common:actions.create")}
      pendingLabel={t("common:actions.creating")}
      isPending={mutation.isPending}
      error={mutation.error}
      onSubmit={submit}
      onClosed={() => {
        setForm(emptyForm(defaultSubtype));
        mutation.reset();
      }}
    >
      <div className="grid gap-2">
        <Label htmlFor="display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
          placeholder={t("liabilities:placeholders.displayName")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="subtype">{t("liabilities:fields.subtype")}</Label>
          <Select
            id="subtype"
            value={form.subtype}
            onChange={(e) =>
              setForm({
                ...form,
                subtype: e.target.value as "personal" | "institutional",
              })
            }
          >
            <option value="personal">{t("liabilities:subtypes.personal")}</option>
            <option value="institutional">{t("liabilities:subtypes.institutional")}</option>
          </Select>
        </div>
        <div className="grid gap-2">
          <Label htmlFor="native_currency">{t("common:fields.currency")}</Label>
          <Input
            id="native_currency"
            required
            value={form.native_currency}
            onChange={(e) =>
              setForm({
                ...form,
                native_currency: e.target.value.toUpperCase(),
              })
            }
            placeholder={t("liabilities:placeholders.currency")}
            maxLength={3}
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="counterparty_name">{t("liabilities:fields.counterparty")}</Label>
        <Input
          id="counterparty_name"
          required
          value={form.counterparty_name}
          onChange={(e) => setForm({ ...form, counterparty_name: e.target.value })}
          placeholder={t("liabilities:placeholders.counterparty")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="principal">{t("liabilities:fields.principal")}</Label>
          <Input
            id="principal"
            inputMode="decimal"
            value={form.principal}
            onChange={(e) => setForm({ ...form, principal: e.target.value })}
            placeholder={t("liabilities:placeholders.principal")}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="interest_rate">{t("liabilities:fields.interestRate")}</Label>
          <Input
            id="interest_rate"
            inputMode="decimal"
            value={form.interest_rate}
            onChange={(e) => setForm({ ...form, interest_rate: e.target.value })}
            placeholder={t("liabilities:placeholders.interestRate")}
          />
        </div>
      </div>

      {/*
        Term / start date / end date. Three columns inside the dialog leaves
        ~100px each at 390px, which truncates the year off both date fields
        ("mm/dd/" with the yy cut) — a straight breach of the ADR-0050 bar, so
        this row stacks on phones. The two-column rows elsewhere in these forms
        deliberately do not: at ~157px they were verified to render the full
        date and its picker icon, and merely-cramped stays single-layout.
      */}
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <div className="grid gap-2">
          <Label htmlFor="term_months">{t("liabilities:fields.term")}</Label>
          <Input
            id="term_months"
            inputMode="numeric"
            value={form.term_months}
            onChange={(e) => setForm({ ...form, term_months: e.target.value })}
            placeholder={t("liabilities:placeholders.term")}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="start_date">{t("liabilities:fields.startDate")}</Label>
          <Input
            id="start_date"
            type="date"
            max="9999-12-31"
            value={form.start_date}
            onChange={(e) => setForm({ ...form, start_date: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="maturity_date">{t("liabilities:fields.maturityDate")}</Label>
          <Input
            id="maturity_date"
            type="date"
            max="9999-12-31"
            value={form.maturity_date}
            onChange={(e) => setForm({ ...form, maturity_date: e.target.value })}
          />
        </div>
      </div>

      <OwnershipField
        idPrefix="liability_create"
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={effectiveSoleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix="liability_create"
        group="liability"
        value={form.entry_type}
        onChange={(v) => setForm({ ...form, entry_type: v })}
      />

      <div className="grid gap-2">
        <Label htmlFor="description">{t("common:fields.description")}</Label>
        <Input
          id="description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>
    </PositionFormDialog>
  );
}
