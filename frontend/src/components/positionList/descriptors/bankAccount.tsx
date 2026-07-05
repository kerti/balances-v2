// A descriptor is a declarative data module, not an HMR component module; it
// legitimately co-locates its small slot components (the headline) with its
// wiring, so the "only export components" fast-refresh rule doesn't apply.
/* eslint-disable react-refresh/only-export-components */
import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { ListHeadline } from "@/components/ListHeadline";
import { CreateBankAccountDialog } from "@/components/CreateBankAccountDialog";
import { EditBankAccountDialog } from "@/components/EditBankAccountDialog";
import {
  useBankAccounts,
  useDeleteBankAccount,
  useImportCreateBankAccount,
} from "@/hooks/useBankAccounts";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession, type Me } from "@/hooks/useSession";
import { ownershipLabel } from "@/lib/ownership";
import { activeCurrencyTotals } from "@/lib/totals";
import type { PositionListDescriptor } from "@/components/positionList/types";
import type { BankAccountListItem, HouseholdMember } from "@/api/types";

// Bank account, expressed as a Position list descriptor (ADR-0043). This is the
// tracer type: it replaces `BankAccountsScreen` + `BankAccountListRow`. The
// only group-specific surface is the ownership column (which needs the
// household member list + current user to render a privacy-safe label,
// INV-PRESENTATION-03) and the create/edit dialogs.

type BankAccountCtx = {
  members: HouseholdMember[] | undefined;
  currentUser: Me | null | undefined;
};

// The per-currency active-balance headline. A component (not an inline slot)
// so it can read its own copy via `t`.
function BankAccountListHeadline({ items }: { items: BankAccountListItem[] }) {
  const { t } = useTranslation("assets");
  const { totals, count } = useMemo(
    () =>
      activeCurrencyTotals(
        items.map((item) => ({
          status: item.asset.status,
          snapshot: item.latest_snapshot,
        })),
      ),
    [items],
  );
  return (
    <ListHeadline
      totals={totals}
      count={count}
      label={t("bankAccount.totalBalance")}
      noun={t("bankAccount.noun")}
      nounPlural={t("bankAccount.nounPlural")}
      testId="bank-accounts-total"
    />
  );
}

export const bankAccountDescriptor: PositionListDescriptor<
  BankAccountListItem,
  BankAccountCtx
> = {
  entityKey: "bankAccount",
  testIdPrefix: "bank-account",
  group: "assets",
  i18nNamespaces: ["assets", "common", "errors"],
  defaultSortKey: "name",

  keys: {
    listTitle: "assets:bankAccount.listTitle",
    listSubtitle: "assets:bankAccount.listSubtitle",
    emptyTitle: "assets:bankAccount.emptyTitle",
    emptyBody: "assets:bankAccount.emptyBody",
    noun: "assets:bankAccount.noun",
    nounPlural: "assets:bankAccount.nounPlural",
    valueLabel: "assets:bankAccount.sortLatestBalance",
    rowActions: "assets:bankAccount.rowActions",
    deleteTitle: "assets:bankAccount.deleteTitle",
  },

  useList: useBankAccounts,
  useDelete: useDeleteBankAccount,
  useImport: useImportCreateBankAccount,
  useExtraContext: (): BankAccountCtx => {
    const { data: members } = useHouseholdMembers();
    const { data: currentUser } = useSession();
    return { members, currentUser };
  },

  getId: (item) => item.asset.id,
  getName: (item) => item.asset.display_name,
  getStatus: (item) => item.asset.status,
  getSnapshot: (item) => item.latest_snapshot,
  getSecondary: (item, t) =>
    t("assets:bankAccount.detailHeaderLine", {
      bankName: item.details.bank_name,
      accountNumber: item.details.account_number,
      accountType: t(
        `assets:bankAccount.accountTypes.${item.details.account_type}`,
      ),
    }),
  deleteDescription: (item, t) =>
    t("assets:bankAccount.deleteRowDescription", {
      name: item.asset.display_name,
    }),

  extraColumns: [
    {
      id: "ownership",
      labelKey: "common:tableHeaders.ownership",
      slot: "main",
      render: (item, ctx) =>
        ownershipLabel(
          item.asset.ownership_type,
          item.asset.sole_owner_user_id,
          ctx.members,
          ctx.currentUser,
        ),
      sort: {
        key: "ownership",
        type: "text",
        value: (item, ctx) =>
          ownershipLabel(
            item.asset.ownership_type,
            item.asset.sole_owner_user_id,
            ctx.members,
            ctx.currentUser,
          ),
      },
    },
  ],

  renderHeadline: (items) => <BankAccountListHeadline items={items} />,
  renderCreateDialog: () => <CreateBankAccountDialog />,
  renderEditDialog: (item, props) => (
    <EditBankAccountDialog
      key={item.asset.id}
      account={{ asset: item.asset, details: item.details }}
      {...props}
    />
  ),
};
