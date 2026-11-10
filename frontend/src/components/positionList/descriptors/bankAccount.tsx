import { CreateBankAccountDialog } from "@/components/dialogs/CreateBankAccountDialog";
import { EditBankAccountDialog } from "@/components/dialogs/EditBankAccountDialog";
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
export const bankAccountDescriptor = nonInvestmentDescriptor<BankAccountListItem>({
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
  },
  useList: useBankAccounts,
  useDelete: useDeleteBankAccount,
  useImport: useImportCreateBankAccount,
  entity: (item) => item.asset,
  getSnapshot: (item) => item.latest_snapshot,
  // A blank account number (#843) drops out with its separator, not a dash.
  getSecondary: (item, t) =>
    t(
      item.details.account_number
        ? "assets:bankAccount.detailHeaderLine"
        : "assets:bankAccount.detailHeaderLineNoNumber",
      {
        bankName: item.details.bank_name,
        accountNumber: item.details.account_number,
        accountType: t(`assets:bankAccount.accountTypes.${item.details.account_type}`),
      },
    ),
  headlineLabelKey: "assets:bankAccount.totalBalance",
  headlineTestId: "bank-accounts-total",
  renderCreateDialog: (onCreated) => <CreateBankAccountDialog onCreated={onCreated} />,
  renderEditDialog: (item, props) => (
    <EditBankAccountDialog
      key={item.asset.id}
      account={{ asset: item.asset, details: item.details }}
      {...props}
    />
  ),
});
