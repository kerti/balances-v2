import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { TableHead, TableRow } from "@/components/ui/table";
import { AccruedInterestSnapshotRow } from "@/components/common/AccruedInterestSnapshotRow";
import { TransactionRow } from "@/components/common/TransactionRow";
import { InvestmentHeadline } from "@/components/common/InvestmentHeadline";
import { CreateAccruedInterestSnapshotDialog } from "@/components/dialogs/CreateAccruedInterestSnapshotDialog";
import { ImportSnapshotsDialog } from "@/components/dialogs/ImportSnapshotsDialog";
import { CreateTradeTransactionDialog } from "@/components/dialogs/CreateTradeTransactionDialog";
import { CreateCashIncomeTransactionDialog } from "@/components/dialogs/CreateCashIncomeTransactionDialog";
import { CreateFeeTransactionDialog } from "@/components/dialogs/CreateFeeTransactionDialog";
import { CreateMaturityTransactionDialog } from "@/components/dialogs/CreateMaturityTransactionDialog";
import { EditBondDialog } from "@/components/dialogs/EditBondDialog";
import { useBond, useDeleteBond } from "@/hooks/useInvestments";
import {
  useInvestmentSnapshots,
  useCreateInvestmentSnapshot,
  useUpdateInvestmentSnapshot,
  useDeleteInvestmentSnapshot,
  useImportInvestmentSnapshots,
  investmentImportTemplateUrl,
  bondExportUrl,
} from "@/hooks/useInvestmentSnapshots";
import {
  useInvestmentTransactions,
  useCreateInvestmentTransaction,
  useUpdateInvestmentTransaction,
  useDeleteInvestmentTransaction,
} from "@/hooks/useInvestmentTransactions";
import { isActiveStatus } from "@/lib/lifecycle";
import { computeCostBasis, costBasisSeries } from "@/lib/costBasis";
import { formatCurrency, formatDate } from "@/lib/format";
import { matchesTxnSearch } from "@/lib/transactionSearch";
import { erasedSection } from "@/components/detail/types";
import type { InfoField } from "@/components/detail/types";
import type { Bond, InvestmentSnapshot, InvestmentTransaction } from "@/api/types";
import type { DetailDescriptor, HistorySectionSpec } from "@/components/detail/types";

const PAGE_SIZE = 12;

// The per-render context for the Bond detail (ADR-0051, A5 — accrued
// investments). Mirrors the A3 Stock mechanism: the transaction ledger + its
// mutations + the search state live here — the "has-transactions" axis — so the
// core never sees a transaction. Bond's coupon income event rides the same
// cash-income dialog stocks use for dividends.
type BondCtx = {
  transactions: InvestmentTransaction[] | undefined;
  createTransactionMutation: ReturnType<typeof useCreateInvestmentTransaction>;
  updateTransactionMutation: ReturnType<typeof useUpdateInvestmentTransaction>;
  deleteTransactionMutation: ReturnType<typeof useDeleteInvestmentTransaction>;
  txnSearch: string;
  setTxnSearch: (value: string) => void;
  quantityUnit: string;
};

// Bond detail, expressed on the generic `PositionDetailScreen` (ADR-0051, A5 —
// the accrued snapshot shape, S3). The accrued renderer (`AccruedInterestSnapshotRow`,
// already built for entry #506) flows through `HistorySection.renderRow`; the
// primitive never inspects its columns. Cost always replays from the ledger
// (every bond carries a Buy at placement, #27), so `totalCost` is descriptor
// wiring via `lib/costBasis` exactly like the qty×price investments. The
// coupon/maturity events fold into the shared transaction section; the
// coupon-disposition and face-value quirks stay in the descriptor's slots, never
// the core.
export const bondDescriptor: DetailDescriptor<Bond, BondCtx, InvestmentSnapshot> = {
  entityKey: "bond",
  testIdPrefix: "bond",
  group: "investments",
  tagGroup: "investment",
  listKey: "bonds",
  i18nNamespaces: ["investments", "common", "errors"],
  keys: {
    detailsCardTitle: "investments:bond.detailsCardTitle",
    detailsCardLine: "investments:bond.detailsCardLine",
    chartTitle: "investments:snapshotsCard.chartTitle",
    chartDescription: "investments:snapshotsCard.chartDescriptionTotal",
    snapshotsTitle: "investments:snapshotsCard.title",
    snapshotsDescription: "investments:bond.snapshotsDescription",
    snapshotsEmpty: "investments:bond.snapshotsEmpty",
    deleteTitle: "investments:bond.deleteTitle",
    deleteDescription: "investments:bond.deleteDetailDescription",
  },
  tourKeyPrefix: "investments:bond",
  // Bond's regions exceed the five standard anchors — investment headline +
  // transaction table — so it overrides the whole tour list.
  tourSteps: (t) => [
    {
      element: '[data-testid="tour-overview"]',
      title: t("investments:bond.tour.overviewTitle"),
      description: t("investments:bond.tour.overviewBody"),
    },
    {
      element: '[data-testid="investment-headline"]',
      title: t("investments:bond.tour.headlineTitle"),
      description: t("investments:bond.tour.headlineBody"),
    },
    {
      element: '[data-testid="tour-actions"]',
      title: t("investments:bond.tour.actionsTitle"),
      description: t("investments:bond.tour.actionsBody"),
    },
    {
      element: '[data-testid="tour-details"]',
      title: t("investments:bond.tour.detailsTitle"),
      description: t("investments:bond.tour.detailsBody"),
    },
    {
      element: '[data-testid="tour-chart"]',
      title: t("investments:bond.tour.chartTitle"),
      description: t("investments:bond.tour.chartBody"),
    },
    {
      element: '[data-testid="tour-snapshots"]',
      title: t("investments:bond.tour.snapshotsTitle"),
      description: t("investments:bond.tour.snapshotsBody"),
    },
    {
      element: '[data-testid="tour-transactions"]',
      title: t("investments:bond.tour.transactionsTitle"),
      description: t("investments:bond.tour.transactionsBody"),
    },
  ],

  useEntity: useBond,
  useDelete: useDeleteBond,
  exportUrl: bondExportUrl,
  getAsset: (entity) => entity.investment,

  useDetailContext: (assetId) => {
    const { t } = useTranslation(["investments"]);
    const { data: transactions } = useInvestmentTransactions(assetId);
    // Maturity flips the parent to 'matured' + upserts a close snapshot, so the
    // create hook carries the "bonds" detailKey to refresh detail/snapshots/list.
    const createTransactionMutation = useCreateInvestmentTransaction(assetId, "bonds");
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
      quantityUnit: t("investments:bond.quantityUnit"),
    };
  },

  headerSecondary: (entity, _ctx, t) => {
    const bondTypeLabel = t(
      entity.details.bond_type === "govt_primary"
        ? "investments:bond.bondType.govt_primary"
        : "investments:bond.bondType.secondary_market",
    );
    return [entity.details.series_code, bondTypeLabel, entity.details.issuer]
      .filter(Boolean)
      .join(" · ");
  },
  infoFields: (entity, _ctx, t) => {
    const { investment, details } = entity;
    const couponPct = Number(details.coupon_rate).toFixed(2);
    const frequencyLabel = t(`investments:bond.couponFrequency.${details.coupon_frequency}`);
    const fields: InfoField[] = [
      {
        label: t("investments:bond.faceValueLabel"),
        value: formatCurrency(entity.outstanding_face, investment.native_currency),
      },
      {
        label: t("investments:bond.couponLabel"),
        value: t("investments:bond.couponValue", {
          rate: couponPct,
          frequency: frequencyLabel,
        }),
      },
      {
        label: t("investments:bond.maturityLabel"),
        value: formatDate(details.maturity_date),
      },
      {
        label: t("investments:bond.fields.couponDisposition"),
        value: t(`investments:bond.couponDisposition.${details.coupon_disposition}`),
      },
    ];
    return fields;
  },

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
      const { data: bond } = useBond(assetId);
      const { data: snapshots } = useInvestmentSnapshots(assetId);
      const createMutation = useCreateInvestmentSnapshot(assetId, "bonds");
      const updateMutation = useUpdateInvestmentSnapshot(assetId, "bonds");
      const deleteMutation = useDeleteInvestmentSnapshot(assetId, "bonds");
      const importMutation = useImportInvestmentSnapshots(assetId, "bonds");
      return {
        snapshots,
        header: (
          <TableRow>
            <TableHead>{t("investments:snapshotsCard.monthHeader")}</TableHead>
            <TableHead className="text-right">
              {t("investments:snapshotsCard.principalHeader")}
            </TableHead>
            <TableHead className="text-right">
              {t("investments:snapshotsCard.accruedHeader")}
            </TableHead>
            <TableHead className="text-right">
              {t("investments:snapshotsCard.totalValueHeader")}
            </TableHead>
            <TableHead>{t("investments:snapshotsCard.notesHeader")}</TableHead>
            <TableHead className="w-12"></TableHead>
          </TableRow>
        ),
        renderRow: (snapshot) => (
          <AccruedInterestSnapshotRow
            key={snapshot.id}
            snapshot={snapshot}
            updateMutation={updateMutation}
            deleteMutation={deleteMutation}
          />
        ),
        renderCreateControls: (currency) => (
          <>
            <CreateAccruedInterestSnapshotDialog
              currency={currency}
              mutation={createMutation}
              couponDisposition={bond?.details.coupon_disposition}
              carryover={
                snapshots?.[0]
                  ? {
                      amount: snapshots[0].amount,
                      accrued_interest: snapshots[0].accrued_interest,
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

  historySections: (entity, ctx, _snapshots, t) => {
    const currency = entity.investment.native_currency;
    const active = isActiveStatus(entity.investment.status);
    const allTxns = ctx.transactions ?? [];
    const filtered = allTxns.filter((tx) => matchesTxnSearch(tx, ctx.txnSearch));
    // Lifetime sums of coupon income and trading fees — the "yield-to-date" glance
    // from #14. Maturity payouts are terminal, not recurring income, so excluded.
    const txnSum = (kind: "coupon" | "fee"): number =>
      allTxns.reduce(
        (acc, tx) => (tx.transaction_type === kind ? acc + Number(tx.amount) : acc),
        0,
      );
    const totalCoupons = txnSum("coupon");
    const totalFees = txnSum("fee");

    const txnSection: HistorySectionSpec<InvestmentTransaction> = {
      testId: "tour-transactions",
      title: t("investments:transactions.cardTitle"),
      description: t("investments:bond.transactionsDescription"),
      emptyText:
        allTxns.length === 0
          ? t("investments:bond.transactionsEmpty")
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
            txnType="coupon"
            mutation={ctx.createTransactionMutation}
          />
          <CreateFeeTransactionDialog
            currency={currency}
            quantityUnit={ctx.quantityUnit}
            mutation={ctx.createTransactionMutation}
          />
          <CreateMaturityTransactionDialog
            currency={currency}
            maturityDate={entity.details.maturity_date?.slice(0, 10)}
            mutation={ctx.createTransactionMutation}
          />
        </>
      ) : undefined,
      // The coupon/fee running totals + the search box share one toolbar row,
      // exactly as the hand-written page laid them out; the primitive renders it
      // above the table but never reads it.
      toolbar:
        allTxns.length > 0 ? (
          <div className="flex flex-wrap items-center justify-between gap-4 border-b px-6 py-3">
            <div className="flex flex-wrap gap-x-6 gap-y-1 text-sm">
              <div>
                <span className="text-muted-foreground">
                  {t("investments:bond.totalCouponsLabel")}
                </span>{" "}
                <span className="tabular-nums">
                  {formatCurrency(totalCoupons.toString(), currency)}
                </span>
              </div>
              <div>
                <span className="text-muted-foreground">
                  {t("investments:bond.totalFeesLabel")}
                </span>{" "}
                <span className="tabular-nums">
                  {formatCurrency(totalFees.toString(), currency)}
                </span>
              </div>
            </div>
            <Input
              data-testid="txn-search"
              placeholder={t("investments:transactions.searchPlaceholder")}
              value={ctx.txnSearch}
              onChange={(e) => ctx.setTxnSearch(e.target.value)}
              className="max-w-xs"
            />
          </div>
        ) : undefined,
    };

    return [erasedSection(txnSection)];
  },

  renderEditDialog: (entity, props) => (
    <EditBondDialog key={entity.investment.id} bond={entity} {...props} />
  ),
};
