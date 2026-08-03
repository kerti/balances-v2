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
import { EditStockDialog } from "@/components/dialogs/EditStockDialog";
import { useStock, useDeleteStock } from "@/hooks/useInvestments";
import {
  useInvestmentSnapshots,
  useCreateInvestmentSnapshot,
  useUpdateInvestmentSnapshot,
  useDeleteInvestmentSnapshot,
  useImportInvestmentSnapshots,
  investmentImportTemplateUrl,
  stockExportUrl,
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
import type { Stock, InvestmentSnapshot, InvestmentTransaction } from "@/api/types";
import type { DetailDescriptor, HistorySectionSpec } from "@/components/detail/types";

const PAGE_SIZE = 12;

// The per-render context for the Stock detail (ADR-0051, A3). The investment
// transaction ledger + its mutations + the search state live here — the
// "has-transactions" axis the amount-only linchpin (#525) never exercised. The
// core calls this once, top-level, and hands it to the headline / chart-cost /
// transaction-section slots; the core itself never sees a transaction.
type StockCtx = {
  transactions: InvestmentTransaction[] | undefined;
  createTransactionMutation: ReturnType<typeof useCreateInvestmentTransaction>;
  updateTransactionMutation: ReturnType<typeof useUpdateInvestmentTransaction>;
  deleteTransactionMutation: ReturnType<typeof useDeleteInvestmentTransaction>;
  txnSearch: string;
  setTxnSearch: (value: string) => void;
  quantityUnit: string;
};

// Stock detail, expressed on the generic `PositionDetailScreen` (ADR-0051, A3 —
// the investment mechanism). Stock is the richest investment type; it proves the
// two investment-only pieces end-to-end: the `renderHeadline` slot (shared
// `InvestmentHeadline` fed cost-basis wiring via `lib/costBasis`) and a
// multi-section `HistorySection` (qty×price snapshots + a transaction ledger with
// its own search toolbar + reconcile banner). No `details.*` field reaches the
// core — the ticker/exchange ride the header line, cost basis is descriptor
// wiring, and the transactions live in `StockCtx`.
export const stockDescriptor: DetailDescriptor<Stock, StockCtx, InvestmentSnapshot> = {
  entityKey: "stock",
  testIdPrefix: "stock",
  group: "investments",
  tagGroup: "investment",
  listKey: "stocks",
  investmentSubtype: "stock",
  i18nNamespaces: ["investments", "common", "errors"],
  keys: {
    detailsCardTitle: "investments:stock.detailsCardTitle",
    detailsCardLine: "investments:stock.detailsCardLine",
    chartTitle: "investments:snapshotsCard.chartTitle",
    chartDescription: "investments:snapshotsCard.chartDescription",
    snapshotsTitle: "investments:snapshotsCard.title",
    snapshotsDescription: "investments:stock.snapshotsDescription",
    snapshotsEmpty: "investments:stock.snapshotsEmpty",
    deleteTitle: "investments:stock.deleteTitle",
    deleteDescription: "investments:stock.deleteDetailDescription",
  },
  tourKeyPrefix: "investments:stock",
  // Stock's regions exceed the five standard anchors — it adds the investment
  // headline and the transaction table — so it overrides the whole tour list.
  // Each step still points at an anchor the core or a populated slot renders.
  tourSteps: (t) => [
    {
      element: '[data-testid="tour-overview"]',
      title: t("investments:stock.tour.overviewTitle"),
      description: t("investments:stock.tour.overviewBody"),
    },
    {
      element: '[data-testid="investment-headline"]',
      title: t("investments:stock.tour.headlineTitle"),
      description: t("investments:stock.tour.headlineBody"),
    },
    {
      element: '[data-testid="tour-actions"]',
      title: t("investments:stock.tour.actionsTitle"),
      description: t("investments:stock.tour.actionsBody"),
    },
    {
      element: '[data-testid="tour-details"]',
      title: t("investments:stock.tour.detailsTitle"),
      description: t("investments:stock.tour.detailsBody"),
    },
    {
      element: '[data-testid="tour-chart"]',
      title: t("investments:stock.tour.chartTitle"),
      description: t("investments:stock.tour.chartBody"),
    },
    {
      element: '[data-testid="tour-snapshots"]',
      title: t("investments:stock.tour.snapshotsTitle"),
      description: t("investments:stock.tour.snapshotsBody"),
    },
    {
      element: '[data-testid="tour-transactions"]',
      title: t("investments:stock.tour.transactionsTitle"),
      description: t("investments:stock.tour.transactionsBody"),
    },
  ],

  useEntity: useStock,
  useDelete: useDeleteStock,
  exportUrl: stockExportUrl,
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
      quantityUnit: t("investments:stock.quantityUnit"),
    };
  },

  // Ticker + exchange move into the details card as a tight, label-less cluster
  // (ADR-0051 Phase B) — the ticker reads as the primary identifier, the exchange
  // muted beneath it — so the card's left column carries the identity the other
  // qty×price types now share, rather than floating it in the H1 subtitle.
  identityCluster: (entity) => (
    <IdentityCluster lines={[entity.details.ticker, entity.details.exchange]} />
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
      const createMutation = useCreateInvestmentSnapshot(assetId, "stocks");
      const updateMutation = useUpdateInvestmentSnapshot(assetId, "stocks");
      const deleteMutation = useDeleteInvestmentSnapshot(assetId, "stocks");
      const importMutation = useImportInvestmentSnapshots(assetId, "stocks");
      const quantityUnit = t("investments:stock.quantityUnit");
      return {
        snapshots,
        header: (
          <TableRow>
            <TableHead>{t("investments:snapshotsCard.monthHeader")}</TableHead>
            <TableHead className="text-right">{t("investments:stock.quantityHeader")}</TableHead>
            <TableHead className="text-right">{t("investments:stock.priceHeader")}</TableHead>
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
        ),
        renderImportControl: (currency) => (
          <ImportSnapshotsDialog
            templateUrl={investmentImportTemplateUrl(assetId)}
            mutation={importMutation}
            currency={currency}
          />
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
      description: t("investments:stock.transactionsDescription"),
      emptyText:
        allTxns.length === 0
          ? t("investments:stock.transactionsEmpty")
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
        // Two rows (#542): row 1 = trades (Buy leads as primary), row 2 = cash
        // flows (dividend income + fees). Stacked on phones, inline from 768px.
        <div className="flex w-full flex-col gap-2 md:w-auto md:flex-row md:flex-wrap md:items-center">
          <div className="flex gap-2 max-md:[&>*]:flex-1" data-testid="txn-trades-row">
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
          </div>
          <div className="flex gap-2 max-md:[&>*]:flex-1" data-testid="txn-cashflow-row">
            <CreateCashIncomeTransactionDialog
              currency={currency}
              txnType="dividend"
              mutation={ctx.createTransactionMutation}
            />
            <CreateFeeTransactionDialog
              currency={currency}
              quantityUnit={ctx.quantityUnit}
              mutation={ctx.createTransactionMutation}
            />
          </div>
        </div>
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
            {t("investments:stock.reconcileWarning", {
              actual: recon.actual,
              expected: recon.expected,
            })}
          </div>
        ) : undefined,
    };

    return [erasedSection(txnSection)];
  },

  renderEditDialog: (entity, props) => (
    <EditStockDialog key={entity.investment.id} stock={entity} {...props} />
  ),
};
