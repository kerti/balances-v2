import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { TableHead, TableRow } from "@/components/ui/table";
import { QuantityPriceSnapshotRow } from "@/components/common/QuantityPriceSnapshotRow";
import { QuantityPriceSnapshotCard } from "@/components/common/QuantityPriceSnapshotCard";
import { TransactionRow } from "@/components/common/TransactionRow";
import { TransactionCard } from "@/components/common/TransactionCard";
import { InvestmentHeadline } from "@/components/common/InvestmentHeadline";
import { RiskProfileBadge } from "@/components/common/RiskProfileBadge";
import { IdentityCluster } from "@/components/detail/IdentityCluster";
import { CreateQuantityPriceSnapshotDialog } from "@/components/dialogs/CreateQuantityPriceSnapshotDialog";
import { ImportSnapshotsDialog } from "@/components/dialogs/ImportSnapshotsDialog";
import { CreateTradeTransactionDialog } from "@/components/dialogs/CreateTradeTransactionDialog";
import { CreateCashIncomeTransactionDialog } from "@/components/dialogs/CreateCashIncomeTransactionDialog";
import { CreateFeeTransactionDialog } from "@/components/dialogs/CreateFeeTransactionDialog";
import { EditMutualFundDialog } from "@/components/dialogs/EditMutualFundDialog";
import { useMutualFund, useDeleteMutualFund } from "@/hooks/useInvestments";
import {
  useInvestmentSnapshots,
  useCreateInvestmentSnapshot,
  useUpdateInvestmentSnapshot,
  useDeleteInvestmentSnapshot,
  useImportInvestmentSnapshots,
  investmentImportTemplateUrl,
  mutualFundExportUrl,
} from "@/hooks/useInvestmentSnapshots";
import {
  useInvestmentTransactions,
  useCreateInvestmentTransaction,
  useUpdateInvestmentTransaction,
  useDeleteInvestmentTransaction,
} from "@/hooks/useInvestmentTransactions";
import { isActiveStatus } from "@/lib/lifecycle";
import { computeCostBasis, costBasisSeries } from "@/lib/costBasis";
import { reconcileQuantity } from "@/lib/reconciliation";
import { matchesTxnSearch } from "@/lib/transactionSearch";
import { erasedSection } from "@/components/detail/types";
import type { MutualFund, InvestmentSnapshot, InvestmentTransaction } from "@/api/types";
import type { DetailDescriptor, HistorySectionSpec } from "@/components/detail/types";

const PAGE_SIZE = 12;

// The per-render context for the MutualFund detail (ADR-0051, A4). Mirrors the
// A3 Stock mechanism: the investment transaction ledger + its mutations + the
// search state live here — the "has-transactions" axis — so the core never sees
// a transaction.
type MutualFundCtx = {
  transactions: InvestmentTransaction[] | undefined;
  createTransactionMutation: ReturnType<typeof useCreateInvestmentTransaction>;
  updateTransactionMutation: ReturnType<typeof useUpdateInvestmentTransaction>;
  deleteTransactionMutation: ReturnType<typeof useDeleteInvestmentTransaction>;
  txnSearch: string;
  setTxnSearch: (value: string) => void;
  quantityUnit: string;
};

// MutualFund detail, expressed on the generic `PositionDetailScreen` (ADR-0051,
// A4 — qty×price completion). A mechanical repeat of the A3 Stock descriptor:
// the `renderHeadline` slot (shared `InvestmentHeadline` fed cost-basis wiring)
// and a multi-section `HistorySection` (units×NAV snapshots + a transaction
// ledger). No `details.*` field reaches the core — the fund code/manager/type
// ride the header line, cost basis is descriptor wiring, transactions live in
// `MutualFundCtx`.
export const mutualFundDescriptor: DetailDescriptor<MutualFund, MutualFundCtx, InvestmentSnapshot> =
  {
    entityKey: "mutualFund",
    testIdPrefix: "mutual-fund",
    group: "investments",
    tagGroup: "investment",
    listKey: "mutual-funds",
    i18nNamespaces: ["investments", "common", "errors"],
    keys: {
      detailsCardTitle: "investments:mutualFund.detailsCardTitle",
      detailsCardLine: "investments:mutualFund.detailsCardLine",
      chartTitle: "investments:snapshotsCard.chartTitle",
      chartDescription: "investments:snapshotsCard.chartDescription",
      snapshotsTitle: "investments:snapshotsCard.title",
      snapshotsDescription: "investments:mutualFund.snapshotsDescription",
      snapshotsEmpty: "investments:mutualFund.snapshotsEmpty",
      deleteTitle: "investments:mutualFund.deleteTitle",
      deleteDescription: "investments:mutualFund.deleteDetailDescription",
    },
    tourKeyPrefix: "investments:mutualFund",
    // MutualFund's regions exceed the five standard anchors — investment headline
    // + transaction table — so it overrides the whole tour list.
    tourSteps: (t) => [
      {
        element: '[data-testid="tour-overview"]',
        title: t("investments:mutualFund.tour.overviewTitle"),
        description: t("investments:mutualFund.tour.overviewBody"),
      },
      {
        element: '[data-testid="investment-headline"]',
        title: t("investments:mutualFund.tour.headlineTitle"),
        description: t("investments:mutualFund.tour.headlineBody"),
      },
      {
        element: '[data-testid="tour-actions"]',
        title: t("investments:mutualFund.tour.actionsTitle"),
        description: t("investments:mutualFund.tour.actionsBody"),
      },
      {
        element: '[data-testid="tour-details"]',
        title: t("investments:mutualFund.tour.detailsTitle"),
        description: t("investments:mutualFund.tour.detailsBody"),
      },
      {
        element: '[data-testid="tour-chart"]',
        title: t("investments:mutualFund.tour.chartTitle"),
        description: t("investments:mutualFund.tour.chartBody"),
      },
      {
        element: '[data-testid="tour-snapshots"]',
        title: t("investments:mutualFund.tour.snapshotsTitle"),
        description: t("investments:mutualFund.tour.snapshotsBody"),
      },
      {
        element: '[data-testid="tour-transactions"]',
        title: t("investments:mutualFund.tour.transactionsTitle"),
        description: t("investments:mutualFund.tour.transactionsBody"),
      },
    ],

    useEntity: useMutualFund,
    useDelete: useDeleteMutualFund,
    exportUrl: mutualFundExportUrl,
    getAsset: (entity) => entity.investment,

    renderHeaderBadge: (entity) => (
      <RiskProfileBadge profile={entity.investment.risk_profile} compact />
    ),

    useDetailContext: (assetId) => {
      const { t } = useTranslation(["investments"]);
      const { data: transactions } = useInvestmentTransactions(assetId);
      const createTransactionMutation = useCreateInvestmentTransaction(assetId);
      const updateTransactionMutation = useUpdateInvestmentTransaction(assetId);
      const deleteTransactionMutation = useDeleteInvestmentTransaction(assetId);
      const [txnSearch, setTxnSearch] = useState("");
      return {
        transactions,
        createTransactionMutation,
        updateTransactionMutation,
        deleteTransactionMutation,
        txnSearch,
        setTxnSearch,
        quantityUnit: t("investments:mutualFund.quantityUnit"),
      };
    },

    // Fund code + manager + type move into the details card as a tight,
    // label-less cluster (ADR-0051 Phase B) — the code reads as the primary
    // identifier, the manager (optional, drops when absent) + type muted beneath
    // it — filling the card's left column instead of the H1 subtitle.
    identityCluster: (entity, _ctx, t) => (
      <IdentityCluster
        lines={[
          entity.details.fund_code,
          entity.details.fund_manager,
          t(`investments:mutualFund.fundType.short.${entity.details.fund_type}`),
        ]}
      />
    ),
    infoFields: () => [],

    renderHeadline: (entity, ctx, snapshots) => {
      const latest = snapshots && snapshots.length > 0 ? snapshots[0] : null;
      return (
        <InvestmentHeadline
          currency={entity.investment.native_currency}
          latestValue={latest ? Number(latest.amount) : null}
          totalCost={computeCostBasis(ctx.transactions ?? []).cost}
          status={entity.investment.status}
          terminatedAt={entity.investment.terminated_at}
        />
      );
    },

    chartCostSeries: (_entity, ctx, snapshots) =>
      costBasisSeries(snapshots ?? [], ctx.transactions ?? []),

    snapshot: {
      useSectionRender: (assetId) => {
        const { t } = useTranslation(["investments"]);
        const { data: snapshots } = useInvestmentSnapshots(assetId);
        const createMutation = useCreateInvestmentSnapshot(assetId, "mutual-funds");
        const updateMutation = useUpdateInvestmentSnapshot(assetId, "mutual-funds");
        const deleteMutation = useDeleteInvestmentSnapshot(assetId, "mutual-funds");
        const importMutation = useImportInvestmentSnapshots(assetId, "mutual-funds");
        const quantityUnit = t("investments:mutualFund.quantityUnit");
        return {
          snapshots,
          header: (
            <TableRow>
              <TableHead>{t("investments:snapshotsCard.monthHeader")}</TableHead>
              <TableHead className="text-right">
                {t("investments:mutualFund.unitsHeader")}
              </TableHead>
              <TableHead className="text-right">{t("investments:mutualFund.navHeader")}</TableHead>
              <TableHead className="text-right">
                {t("investments:snapshotsCard.totalValueHeader")}
              </TableHead>
              <TableHead>{t("investments:snapshotsCard.notesHeader")}</TableHead>
              <TableHead className="w-12"></TableHead>
            </TableRow>
          ),
          renderRow: (snapshot) => (
            <QuantityPriceSnapshotRow
              key={snapshot.id}
              snapshot={snapshot}
              quantityUnit={quantityUnit}
              updateMutation={updateMutation}
              deleteMutation={deleteMutation}
            />
          ),
          renderCard: (snapshot) => (
            <QuantityPriceSnapshotCard
              key={snapshot.id}
              snapshot={snapshot}
              quantityUnit={quantityUnit}
              updateMutation={updateMutation}
              deleteMutation={deleteMutation}
            />
          ),
          renderCreateControls: (currency) => (
            <>
              <CreateQuantityPriceSnapshotDialog
                currency={currency}
                mutation={createMutation}
                carryover={
                  snapshots?.[0]
                    ? {
                        quantity: snapshots[0].quantity,
                        price_per_unit: snapshots[0].price_per_unit,
                        lastSnapshotMonth: snapshots[0].year_month,
                      }
                    : null
                }
              />
              <ImportSnapshotsDialog
                templateUrl={investmentImportTemplateUrl(assetId)}
                mutation={importMutation}
                currency={currency}
              />
            </>
          ),
        };
      },
    },

    historySections: (entity, ctx, snapshots, t) => {
      const currency = entity.investment.native_currency;
      const active = isActiveStatus(entity.investment.status);
      const latest = snapshots && snapshots.length > 0 ? snapshots[0] : null;
      const recon = reconcileQuantity(latest, ctx.transactions);
      const allTxns = ctx.transactions ?? [];
      const filtered = allTxns.filter((tx) => matchesTxnSearch(tx, ctx.txnSearch));

      const txnSection: HistorySectionSpec<InvestmentTransaction> = {
        testId: "tour-transactions",
        title: t("investments:transactions.cardTitle"),
        description: t("investments:mutualFund.transactionsDescription"),
        emptyText:
          allTxns.length === 0
            ? t("investments:mutualFund.transactionsEmpty")
            : t("investments:transactions.searchEmpty"),
        header: (
          <TableRow>
            <TableHead>{t("investments:transactions.dateHeader")}</TableHead>
            <TableHead>{t("investments:transactions.typeHeader")}</TableHead>
            <TableHead className="text-right">
              {t("investments:transactions.cashImpactHeader")}
            </TableHead>
            <TableHead>{t("investments:transactions.notesHeader")}</TableHead>
            <TableHead className="w-12"></TableHead>
          </TableRow>
        ),
        rows: filtered,
        renderRow: (tx: InvestmentTransaction) => (
          <TransactionRow
            key={tx.id}
            transaction={tx}
            quantityUnit={ctx.quantityUnit}
            updateMutation={ctx.updateTransactionMutation}
            deleteMutation={ctx.deleteTransactionMutation}
          />
        ),
        renderCard: (tx: InvestmentTransaction) => (
          <TransactionCard
            key={tx.id}
            transaction={tx}
            quantityUnit={ctx.quantityUnit}
            updateMutation={ctx.updateTransactionMutation}
            deleteMutation={ctx.deleteTransactionMutation}
          />
        ),
        pageSize: PAGE_SIZE,
        headerActions: active ? (
          <>
            <CreateTradeTransactionDialog
              currency={currency}
              txnType="buy"
              quantityUnit={ctx.quantityUnit}
              mutation={ctx.createTransactionMutation}
            />
            <CreateTradeTransactionDialog
              currency={currency}
              txnType="sell"
              quantityUnit={ctx.quantityUnit}
              mutation={ctx.createTransactionMutation}
            />
            <CreateCashIncomeTransactionDialog
              currency={currency}
              txnType="distribution"
              mutation={ctx.createTransactionMutation}
            />
            <CreateFeeTransactionDialog
              currency={currency}
              quantityUnit={ctx.quantityUnit}
              mutation={ctx.createTransactionMutation}
            />
          </>
        ) : undefined,
        toolbar:
          allTxns.length > 0 ? (
            <div className="border-b px-6 py-3">
              <Input
                data-testid="txn-search"
                placeholder={t("investments:transactions.searchPlaceholder")}
                value={ctx.txnSearch}
                onChange={(e) => ctx.setTxnSearch(e.target.value)}
                className="max-w-xs"
              />
            </div>
          ) : undefined,
        banner:
          recon && !recon.matches ? (
            <div className="mx-6 mb-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900">
              {t("investments:mutualFund.reconcileWarning", {
                actual: recon.actual,
                expected: recon.expected,
              })}
            </div>
          ) : undefined,
      };

      return [erasedSection(txnSection)];
    },

    renderEditDialog: (entity, props) => (
      <EditMutualFundDialog key={entity.investment.id} mutualFund={entity} {...props} />
    ),
  };
