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
import { useCreateMutualFund } from "@/hooks/useInvestments";
import { useSession } from "@/hooks/useSession";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { errorMessage } from "@/lib/errorMessage";
import { RiskProfileSelect } from "@/components/RiskProfileSelect";
import { MutualFundTypeSelect } from "@/components/MutualFundTypeSelect";
import type { RiskProfile, MutualFundType } from "@/api/types";

function emptyForm() {
  return {
    display_name: "",
    description: "",
    ownership_type: "joint" as "sole" | "joint",
    sole_owner_user_id: null as string | null,
    risk_profile: "" as RiskProfile | "",
    native_currency: "IDR",
    fund_code: "",
    fund_manager: "",
    fund_type: "" as MutualFundType | "",
  };
}

export function CreateMutualFundDialog() {
  const { t } = useTranslation(["investments", "common"]);
  const [open, setOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();
  const mutation = useCreateMutualFund();

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function close() {
    setOpen(false);
    setForm(emptyForm());
    mutation.reset();
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!user) return;
    if (!form.risk_profile) return;
    if (!form.fund_type) return;
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id:
          form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        risk_profile: form.risk_profile,
        native_currency: form.native_currency,
        fund_code: form.fund_code,
        fund_manager: form.fund_manager || null,
        fund_type: form.fund_type,
      },
      { onSuccess: close },
    );
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      <DialogTrigger asChild>
        <Button>
          <Plus className="mr-1 size-4" />
          {t("investments:mutualFund.createTrigger")}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("investments:mutualFund.createTitle")}</DialogTitle>
          <DialogDescription>
            {t("investments:mutualFund.createDescription")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid gap-2">
            <Label htmlFor="mf_display_name">
              {t("common:fields.displayName")}
            </Label>
            <Input
              id="mf_display_name"
              required
              value={form.display_name}
              onChange={(e) =>
                setForm({ ...form, display_name: e.target.value })
              }
              placeholder={t("investments:mutualFund.placeholders.displayName")}
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="grid gap-2">
              <Label htmlFor="mf_fund_code">
                {t("investments:mutualFund.fields.fundCode")}
              </Label>
              <Input
                id="mf_fund_code"
                required
                value={form.fund_code}
                onChange={(e) =>
                  setForm({ ...form, fund_code: e.target.value })
                }
                placeholder={t("investments:mutualFund.placeholders.fundCode")}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="mf_fund_manager">
                {t("investments:mutualFund.fields.fundManager")}
              </Label>
              <Input
                id="mf_fund_manager"
                value={form.fund_manager}
                onChange={(e) =>
                  setForm({ ...form, fund_manager: e.target.value })
                }
                placeholder={t(
                  "investments:mutualFund.placeholders.fundManager",
                )}
              />
            </div>
          </div>

          <MutualFundTypeSelect
            idPrefix="mf_create"
            value={form.fund_type}
            onChange={(v) => setForm({ ...form, fund_type: v })}
          />

          <div className="grid gap-2">
            <Label htmlFor="mf_currency">{t("common:fields.currency")}</Label>
            <Input
              id="mf_currency"
              required
              value={form.native_currency}
              onChange={(e) =>
                setForm({
                  ...form,
                  native_currency: e.target.value.toUpperCase(),
                })
              }
              placeholder={t("investments:mutualFund.placeholders.currency")}
              maxLength={3}
            />
          </div>

          <div className="grid gap-2">
            <Label>{t("common:fields.ownership")}</Label>
            <div className="flex gap-4 text-sm">
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="mf_ownership_type"
                  value="joint"
                  checked={form.ownership_type === "joint"}
                  onChange={() => setForm({ ...form, ownership_type: "joint" })}
                />
                {t("investments:ownership.joint")}
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="mf_ownership_type"
                  value="sole"
                  checked={form.ownership_type === "sole"}
                  onChange={() => setForm({ ...form, ownership_type: "sole" })}
                />
                {t("investments:ownership.soleOwner")}
              </label>
            </div>
            {form.ownership_type === "sole" && (
              <select
                aria-label={t("investments:ownership.soleOwnerAria")}
                className="h-9 rounded-md border border-input bg-background px-3 text-sm"
                value={effectiveSoleOwnerID ?? ""}
                onChange={(e) =>
                  setForm({ ...form, sole_owner_user_id: e.target.value })
                }
              >
                {(members ?? []).map((m) => (
                  <option key={m.id} value={m.id}>
                    {preferredName(m)}
                    {user && m.id === user.id
                      ? t("common:ownership.youSuffix")
                      : ""}
                  </option>
                ))}
              </select>
            )}
          </div>

          <div className="grid gap-2">
            <Label htmlFor="mf_description">
              {t("common:fields.description")}
            </Label>
            <Input
              id="mf_description"
              value={form.description}
              onChange={(e) =>
                setForm({ ...form, description: e.target.value })
              }
            />
          </div>

          <RiskProfileSelect
            idPrefix="mf_create"
            value={form.risk_profile}
            onChange={(v) => setForm({ ...form, risk_profile: v })}
          />

          {mutation.error && (
            <p className="text-sm text-destructive">
              {errorMessage(mutation.error)}
            </p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending
                ? t("common:actions.creating")
                : t("common:actions.create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
