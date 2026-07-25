import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { TableHead, TableRow } from "@/components/ui/table";
import { QuantityPriceSnapshotRow } from "@/components/common/QuantityPriceSnapshotRow";
import { QuantityPriceSnapshotCard } from "@/components/common/QuantityPriceSnapshotCard";
import { TransactionRow } from "@/components/common/TransactionRow";
import { InvestmentHeadline } from "@/components/common/InvestmentHeadline";
import { IdentityCluster } from "@/components/detail/IdentityCluster";
import { CreateQuantityPriceSnapshotDialog } from "@/components/dialogs/CreateQuantityPriceSnapshotDialog";
import { ImportSnapshotsDialog } from "@/components/dialogs/ImportSnapshotsDialog";
import { CreateTradeTransactionDialog } from "@/components/dialogs/CreateTradeTransactionDialog";
import { CreateFeeTransactionDialog } from "@/components/dialogs/CreateFeeTransactionDialog";
import { EditGoldDialog } from "@/components/dialogs/EditGoldDialog";
import { useGold, useDeleteGold } from "@/hooks/useInvestments";
import {
  useInvestmentSnapshots,
  useCreateInvestmentSnapshot,
  useUpdateInvestmentSnapshot,
  useDeleteInvestmentSnapshot,
  useImportInvestmentSnapshots,
  investmentImportTemplateUrl,
  goldExportUrl,
} from "@/hooks/useInvestmentSnapshots";
import {
  useInvestmentTransactions,
  useCreateInvestmentTransaction,
  useUpdateInvestmentTransaction,
  useDeleteInvestmentTransaction,
} from "@/hooks/useInvestmentTransactions";
import { isActiveStatus } from "@/lib/lifecycle";
import { computeCostBasis, costBasisSeries } from "@/lib/costBasis";
import { formatGoldPurity } from "@/lib/gold";
import { reconcileQuantity } from "@/lib/reconciliation";
import { matchesTxnSearch } from "@/lib/transactionSearch";
import { erasedSection } from "@/components/detail/types";
import type { Gold, InvestmentSnapshot, InvestmentTransaction } from "@/api/types";
import type { DetailDescriptor, HistorySectionSpec } from "@/components/detail/types";

const PAGE_SIZE = 12;

// The per-render context for the Gold detail (ADR-0051, A4). Mirrors the A3
// Stock mechanism: the investment transaction ledger + its mutations + the
// search state live here — the "has-transactions" axis — so the core never sees
// a transaction. Gold has no cash-income event (no dividend/distribution).
type GoldCtx = {
  transactions: InvestmentTransaction[] | undefined;
  createTransactionMutation: ReturnType<typeof useCreateInvestmentTransaction>;
  updateTransactionMutation: ReturnType<typeof useUpdateInvestmentTransaction>;
  deleteTransactionMutation: ReturnType<typeof useDeleteInvestmentTransaction>;
  txnSearch: string;
  setTxnSearch: (value: string) => void;
  quantityUnit: string;
};

// Gold detail, expressed on the generic `PositionDetailScreen` (ADR-0051, A4 —
// qty×price completion). A mechanical repeat of the A3 Stock descriptor with two
// gold-local nuances kept as descriptor wiring (no mechanism change): trade +
// snapshot dialogs carry a spot-price hint, and there is no cash-income event.
// The form/purity ride the header line; cost basis is descriptor wiring;
// transactions live in `GoldCtx`.
export const goldDescriptor: DetailDescriptor<Gold, GoldCtx, InvestmentSnapshot> = {
  entityKey: "gold",
  testIdPrefix: "gold",
  group: "investments",
  tagGroup: "investment",
  listKey: "golds",
  i18nNamespaces: ["investments", "common", "errors"],
  keys: {
    detailsCardTitle: "investments:gold.detailsCardTitle",
    detailsCardLine: "investments:gold.detailsCardLine",
    chartTitle: "investments:snapshotsCard.chartTitle",
    chartDescription: "investments:snapshotsCard.chartDescription",
    snapshotsTitle: "investments:snapshotsCard.title",
    snapshotsDescription: "investments:gold.snapshotsDescription",
    snapshotsEmpty: "investments:gold.snapshotsEmpty",
    deleteTitle: "investments:gold.deleteTitle",
    deleteDescription: "investments:gold.deleteDetailDescription",
  },
  tourKeyPrefix: "investments:gold",
  // Gold's regions exceed the five standard anchors — investment headline +
  // transaction table — so it overrides the whole tour list.
  tourSteps: (t) => [
    {
      element: '[data-testid="tour-overview"]',
      title: t("investments:gold.tour.overviewTitle"),
      description: t("investments:gold.tour.overviewBody"),
    },
    {
      element: '[data-testid="investment-headline"]',
      title: t("investments:gold.tour.headlineTitle"),
      description: t("investments:gold.tour.headlineBody"),
    },
    {
      element: '[data-testid="tour-actions"]',
      title: t("investments:gold.tour.actionsTitle"),
      description: t("investments:gold.tour.actionsBody"),
    },
    {
      element: '[data-testid="tour-details"]',
      title: t("investments:gold.tour.detailsTitle"),
      description: t("investments:gold.tour.detailsBody"),
    },
    {
      element: '[data-testid="tour-chart"]',
      title: t("investments:gold.tour.chartTitle"),
      description: t("investments:gold.tour.chartBody"),
    },
    {
      element: '[data-testid="tour-snapshots"]',
      title: t("investments:gold.tour.snapshotsTitle"),
      description: t("investments:gold.tour.snapshotsBody"),
    },
    {
      element: '[data-testid="tour-transactions"]',
      title: t("investments:gold.tour.transactionsTitle"),
      description: t("investments:gold.tour.transactionsBody"),
    },
  ],

  useEntity: useGold,
  useDelete: useDeleteGold,
  exportUrl: goldExportUrl,
  getAsset: (entity) => entity.investment,

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
      quantityUnit: t("investments:gold.quantityUnit"),
    };
  },

  // Form + purity move into the details card as a tight, label-less cluster
  // (ADR-0051 Phase B) — the form reads as the primary identifier, the purity
  // muted beneath it — filling the card's left column instead of the H1 subtitle.
  identityCluster: (entity, _ctx, t) => (
    <IdentityCluster
      lines={[
        t(`investments:gold.goldForms.${entity.details.form}`),
        formatGoldPurity(entity.details.purity),
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
      const createMutation = useCreateInvestmentSnapshot(assetId, "golds");
      const updateMutation = useUpdateInvestmentSnapshot(assetId, "golds");
      const deleteMutation = useDeleteInvestmentSnapshot(assetId, "golds");
      const importMutation = useImportInvestmentSnapshots(assetId, "golds");
      const quantityUnit = t("investments:gold.quantityUnit");
      return {
        snapshots,
        header: (
          <TableRow>
            <TableHead>{t("investments:snapshotsCard.monthHeader")}</TableHead>
            <TableHead className="text-right">{t("investments:gold.gramsHeader")}</TableHead>
            <TableHead className="text-right">{t("investments:gold.pricePerGramHeader")}</TableHead>
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
              priceHint={t("investments:gold.snapshotPriceHint")}
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
      description: t("investments:gold.transactionsDescription"),
      emptyText:
        allTxns.length === 0
          ? t("investments:gold.transactionsEmpty")
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
      pageSize: PAGE_SIZE,
      headerActions: active ? (
        <>
          <CreateTradeTransactionDialog
            currency={currency}
            txnType="buy"
            quantityUnit={ctx.quantityUnit}
            priceHint={t("investments:gold.buyPriceHint")}
            mutation={ctx.createTransactionMutation}
          />
          <CreateTradeTransactionDialog
            currency={currency}
            txnType="sell"
            quantityUnit={ctx.quantityUnit}
            priceHint={t("investments:gold.sellPriceHint")}
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
            {t("investments:gold.reconcileWarning", {
              actual: recon.actual,
              expected: recon.expected,
            })}
          </div>
        ) : undefined,
    };

    return [erasedSection(txnSection)];
  },

  renderEditDialog: (entity, props) => (
    <EditGoldDialog key={entity.investment.id} gold={entity} {...props} />
  ),
};
