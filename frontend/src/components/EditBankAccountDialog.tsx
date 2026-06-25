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
import { useUpdateBankAccount } from "@/hooks/useBankAccounts";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { preferredName } from "@/lib/names";
import { useSession } from "@/hooks/useSession";
import { errorMessage } from "@/lib/errorMessage";
import type { BankAccount } from "@/api/types";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  account: BankAccount;
};

// EditBankAccountDialog is controlled by the caller (no DialogTrigger). The
// parent passes open/onOpenChange and the account row to pre-fill from.
export function EditBankAccountDialog({ open, onOpenChange, account }: Props) {
  const { t } = useTranslation(["assets", "common"]);
  const mutation = useUpdateBankAccount(account.asset.id);
  const { data: user } = useSession();
  const { data: members } = useHouseholdMembers();

  const [form, setForm] = useState({
    display_name: account.asset.display_name,
    description: account.asset.description ?? "",
    ownership_type: account.asset.ownership_type,
    sole_owner_user_id: account.asset.sole_owner_user_id,
    bank_name: account.details.bank_name,
    account_number: account.details.account_number,
    account_type: account.details.account_type,
  });

  const effectiveSoleOwnerID = form.sole_owner_user_id ?? user?.id ?? null;

  function submit(e: React.FormEvent) {
    e.preventDefault();
    mutation.mutate(
      {
        display_name: form.display_name,
        description: form.description || null,
        ownership_type: form.ownership_type,
        sole_owner_user_id:
          form.ownership_type === "sole" ? effectiveSoleOwnerID : null,
        bank_name: form.bank_name,
        account_number: form.account_number,
        account_type: form.account_type,
      },
      { onSuccess: () => onOpenChange(false) },
    );
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("assets:bankAccount.editTitle")}</DialogTitle>
          <DialogDescription>
            {t("assets:bankAccount.editDescription")}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid gap-2">
            <Label htmlFor="edit_display_name">
              {t("common:fields.displayName")}
            </Label>
            <Input
              id="edit_display_name"
              required
              value={form.display_name}
              onChange={(e) =>
                setForm({ ...form, display_name: e.target.value })
              }
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="edit_bank_name">
              {t("assets:bankAccount.fields.bankName")}
            </Label>
            <Input
              id="edit_bank_name"
              required
              value={form.bank_name}
              onChange={(e) => setForm({ ...form, bank_name: e.target.value })}
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="edit_account_number">
              {t("assets:bankAccount.fields.accountNumber")}
            </Label>
            <Input
              id="edit_account_number"
              required
              value={form.account_number}
              onChange={(e) =>
                setForm({ ...form, account_number: e.target.value })
              }
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="edit_account_type">
              {t("assets:bankAccount.fields.accountType")}
            </Label>
            <select
              id="edit_account_type"
              className="h-9 rounded-md border border-input bg-background px-3 text-sm"
              value={form.account_type}
              onChange={(e) =>
                setForm({
                  ...form,
                  account_type: e.target.value as typeof form.account_type,
                })
              }
            >
              <option value="savings">
                {t("assets:bankAccount.accountTypes.savings")}
              </option>
              <option value="current">
                {t("assets:bankAccount.accountTypes.current")}
              </option>
              <option value="other">
                {t("assets:bankAccount.accountTypes.other")}
              </option>
            </select>
          </div>

          <div className="grid gap-2">
            <Label>{t("common:fields.ownership")}</Label>
            <div className="flex gap-4 text-sm">
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="edit_ba_ownership_type"
                  value="joint"
                  checked={form.ownership_type === "joint"}
                  onChange={() => setForm({ ...form, ownership_type: "joint" })}
                />
                {t("common:ownership.joint")}
              </label>
              <label className="flex items-center gap-2">
                <input
                  type="radio"
                  name="edit_ba_ownership_type"
                  value="sole"
                  checked={form.ownership_type === "sole"}
                  onChange={() => setForm({ ...form, ownership_type: "sole" })}
                />
                {t("common:ownership.soleOwner")}
              </label>
            </div>
            {form.ownership_type === "sole" && (
              <select
                aria-label={t("common:ownership.soleOwner")}
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
            <Label htmlFor="edit_description">
              {t("common:fields.description")}
            </Label>
            <Input
              id="edit_description"
              value={form.description}
              onChange={(e) =>
                setForm({ ...form, description: e.target.value })
              }
            />
          </div>

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
              {t("common:cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending
                ? t("common:actions.saving")
                : t("common:actions.saveChanges")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
