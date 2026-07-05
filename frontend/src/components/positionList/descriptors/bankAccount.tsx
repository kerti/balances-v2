import { CreateBankAccountDialog } from "@/components/CreateBankAccountDialog";
import { EditBankAccountDialog } from "@/components/EditBankAccountDialog";
import {
  useBankAccounts,
  useDeleteBankAccount,
  useImportCreateBankAccount,
} from "@/hooks/useBankAccounts";
import { nonInvestmentDescriptor } from "@/components/positionList/presets/nonInvestment";
import type { BankAccountListItem } from "@/api/types";

// Bank account, expressed on the non-investment preset (ADR-0043) — the tracer
// type (#330), now sharing the ownership column + active-total headline with
// its siblings. The only group-specific surface is the bank-detail secondary
// line and the create/edit dialogs.
export const bankAccountDescriptor =
  nonInvestmentDescriptor<BankAccountListItem>({
    entityKey: "bankAccount",
    testIdPrefix: "bank-account",
    group: "assets",
    i18nNamespaces: ["assets", "common", "errors"],
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
    entity: (item) => item.asset,
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
    headlineLabelKey: "assets:bankAccount.totalBalance",
    headlineTestId: "bank-accounts-total",
    renderCreateDialog: () => <CreateBankAccountDialog />,
    renderEditDialog: (item, props) => (
      <EditBankAccountDialog
        key={item.asset.id}
        account={{ asset: item.asset, details: item.details }}
        {...props}
      />
    ),
  });
