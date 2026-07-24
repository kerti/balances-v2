import { useTranslation } from "react-i18next";
import { TableHead, TableRow } from "@/components/ui/table";
import { SnapshotRow } from "@/components/common/SnapshotRow";
import { CreateSnapshotDialog } from "@/components/dialogs/CreateSnapshotDialog";
import { ImportSnapshotsDialog } from "@/components/dialogs/ImportSnapshotsDialog";
import { EditBankAccountDialog } from "@/components/dialogs/EditBankAccountDialog";
import { useBankAccount, useDeleteBankAccount } from "@/hooks/useBankAccounts";
import {
  useSnapshots,
  useCreateSnapshot,
  useUpdateSnapshot,
  useDeleteSnapshot,
  useImportSnapshots,
  importTemplateUrl,
  bankAccountExportUrl,
} from "@/hooks/useAssetSnapshots";
import type { BankAccount } from "@/api/types";
import type { DetailDescriptor } from "@/components/detail/types";

// Bank account detail, expressed on the generic `PositionDetailScreen`
// (ADR-0051) — the linchpin (A1). An amount-only asset: no info-grid fields
// (the bank/account/type ride the header secondary line), the S1 snapshot
// renderer (`SnapshotRow`), and no transaction sections. Sets the descriptor API
// the other nine types consume.
export const bankAccountDescriptor: DetailDescriptor<BankAccount> = {
  entityKey: "bankAccount",
  testIdPrefix: "bank-account",
  group: "assets",
  tagGroup: "asset",
  listKey: "bank-accounts",
  i18nNamespaces: ["assets", "common", "errors"],
  keys: {
    detailsCardTitle: "assets:bankAccount.detailsCardTitle",
    detailsCardLine: "assets:bankAccount.detailsCardLine",
    chartTitle: "assets:bankAccount.chartTitle",
    chartDescription: "assets:bankAccount.chartDescription",
    snapshotsTitle: "assets:bankAccount.snapshotsTitle",
    snapshotsDescription: "assets:bankAccount.snapshotsDescription",
    snapshotsEmpty: "assets:bankAccount.snapshotsEmpty",
    deleteTitle: "assets:bankAccount.deleteTitle",
    deleteDescription: "assets:bankAccount.deleteDetailDescription",
  },
  tourKeyPrefix: "assets:bankAccount",

  useEntity: useBankAccount,
  useDelete: useDeleteBankAccount,
  exportUrl: bankAccountExportUrl,
  getAsset: (entity) => entity.asset,

  headerSecondary: (entity, _ctx, t) =>
    t("assets:bankAccount.detailHeaderLine", {
      bankName: entity.details.bank_name,
      accountNumber: entity.details.account_number,
      accountType: t(`assets:bankAccount.accountTypes.${entity.details.account_type}`),
    }),
  infoFields: () => [],

  snapshot: {
    useSectionRender: (assetId) => {
      const { t } = useTranslation(["common"]);
      const { data: snapshots } = useSnapshots(assetId);
      const createMutation = useCreateSnapshot(assetId);
      const updateMutation = useUpdateSnapshot(assetId);
      const deleteMutation = useDeleteSnapshot(assetId);
      const importMutation = useImportSnapshots(assetId);
      return {
        snapshots,
        header: (
          <TableRow>
            <TableHead>{t("common:tableHeaders.month")}</TableHead>
            <TableHead>{t("common:tableHeaders.amount")}</TableHead>
            <TableHead>{t("common:tableHeaders.notes")}</TableHead>
            <TableHead className="w-12"></TableHead>
          </TableRow>
        ),
        renderRow: (snapshot) => (
          <SnapshotRow
            key={snapshot.id}
            snapshot={snapshot}
            updateMutation={updateMutation}
            deleteMutation={deleteMutation}
          />
        ),
        renderCreateControls: (currency) => (
          <>
            <CreateSnapshotDialog
              currency={currency}
              mutation={createMutation}
              carryover={
                snapshots?.[0]
                  ? { amount: snapshots[0].amount, lastSnapshotMonth: snapshots[0].year_month }
                  : null
              }
            />
            <ImportSnapshotsDialog
              templateUrl={importTemplateUrl(assetId)}
              mutation={importMutation}
              currency={currency}
            />
          </>
        ),
      };
    },
  },

  renderEditDialog: (entity, props) => (
    <EditBankAccountDialog key={entity.asset.id} account={entity} {...props} />
  ),
};
