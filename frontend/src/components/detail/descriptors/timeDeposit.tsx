import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate } from "react-router";
import { ArrowDown, ArrowUp, Repeat } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { TableHead, TableRow } from "@/components/ui/table";
import { AccruedInterestSnapshotRow } from "@/components/common/AccruedInterestSnapshotRow";
import { AccruedInterestSnapshotCard } from "@/components/common/AccruedInterestSnapshotCard";
import { TransactionRow } from "@/components/common/TransactionRow";
import { TransactionCard } from "@/components/common/TransactionCard";
import { InvestmentHeadline } from "@/components/common/InvestmentHeadline";
import { RiskProfileBadge } from "@/components/common/RiskProfileBadge";
import { IdentityCluster } from "@/components/detail/IdentityCluster";
import { CreateAccruedInterestSnapshotDialog } from "@/components/dialogs/CreateAccruedInterestSnapshotDialog";
import { ImportSnapshotsDialog } from "@/components/dialogs/ImportSnapshotsDialog";
import { CreateMaturityTransactionDialog } from "@/components/dialogs/CreateMaturityTransactionDialog";
import { CreateTimeDepositDialog } from "@/components/dialogs/CreateTimeDepositDialog";
import { LinkRolloverSuccessorDialog } from "@/components/dialogs/LinkRolloverSuccessorDialog";
import { EditTimeDepositDialog } from "@/components/dialogs/EditTimeDepositDialog";
import { useTimeDeposit, useDeleteTimeDeposit } from "@/hooks/useInvestments";
import {
  useInvestmentSnapshots,
  useCreateInvestmentSnapshot,
  useUpdateInvestmentSnapshot,
  useDeleteInvestmentSnapshot,
  useImportInvestmentSnapshots,
  investmentImportTemplateUrl,
  timeDepositExportUrl,
} from "@/hooks/useInvestmentSnapshots";
import {
  useInvestmentTransactions,
  useCreateInvestmentTransaction,
  useUpdateInvestmentTransaction,
  useDeleteInvestmentTransaction,
} from "@/hooks/useInvestmentTransactions";
import { isActiveStatus } from "@/lib/lifecycle";
import { flatCostSeries } from "@/lib/costBasis";
import { formatCurrency, formatDate } from "@/lib/format";
import { matchesTxnSearch } from "@/lib/transactionSearch";
import { maturityRolloverPrefill } from "@/lib/rollover";
import { routes } from "@/lib/routes";
import { erasedSection } from "@/components/detail/types";
import type { InfoField } from "@/components/detail/types";
import type { TimeDeposit, InvestmentSnapshot, InvestmentTransaction } from "@/api/types";
import type { DetailDescriptor, HistorySectionSpec } from "@/components/detail/types";

const PAGE_SIZE = 12;

// The per-render context for the TimeDeposit detail (ADR-0051, A5 — the 25-Card
// outlier). Carries the transaction ledger + its mutations + the search state
// (the "has-transactions" axis) like the other investments, plus one TD-only
// wire: `selectTimeDeposit`, the rollover-chain navigation. The core never sees
// it — the rollover-chain card lives in a `renderAfterDetails` slot and calls
// this from there. Navigation is descriptor-level app wiring (the core stays
// router-unaware, exactly as it was when App.tsx bridged the callback).
type TimeDepositCtx = {
  transactions: InvestmentTransaction[] | undefined;
  createTransactionMutation: ReturnType<typeof useCreateInvestmentTransaction>;
  updateTransactionMutation: ReturnType<typeof useUpdateInvestmentTransaction>;
  deleteTransactionMutation: ReturnType<typeof useDeleteInvestmentTransaction>;
  txnSearch: string;
  setTxnSearch: (value: string) => void;
  selectTimeDeposit: (id: string) => void;
};

// TimeDeposit detail, expressed on the generic `PositionDetailScreen` (ADR-0051,
// A5 — the accrued shape's outlier). Two things make it the tail of Phase A, both
// kept inside slots rather than leaking into the core:
//   1. `totalCost` is the flat `principal`, not a ledger replay — the one
//      has-transactions type whose cost is a scalar, wired straight into the
//      shared `InvestmentHeadline`.
//   2. The maturity/rollover linkage — the post-maturity callout that offers to
//      spawn (or link) the successor deposit, and the rollover-chain card that
//      navigates the from/into links. Both ride the neutral `renderBeforeDetails`
//      / `renderAfterDetails` slots; the core renders the nodes verbatim and never
//      learns what a rollover is.
// The accrued snapshot renderer + transaction section are the same mechanism the
// Bond slice proves; only the maturity dialog + the two rollover surfaces differ.
export const timeDepositDescriptor: DetailDescriptor<
  TimeDeposit,
  TimeDepositCtx,
  InvestmentSnapshot
> = {
  entityKey: "timeDeposit",
  testIdPrefix: "time-deposit",
  group: "investments",
  tagGroup: "investment",
  listKey: "time-deposits",
  i18nNamespaces: ["investments", "common", "errors"],
  keys: {
    detailsCardTitle: "investments:timeDeposit.detailsCardTitle",
    detailsCardLine: "investments:timeDeposit.detailsCardLine",
    chartTitle: "investments:snapshotsCard.chartTitle",
    chartDescription: "investments:snapshotsCard.chartDescriptionTotal",
    snapshotsTitle: "investments:snapshotsCard.title",
    snapshotsDescription: "investments:timeDeposit.snapshotsDescription",
    snapshotsEmpty: "investments:timeDeposit.snapshotsEmpty",
    deleteTitle: "investments:timeDeposit.deleteTitle",
    deleteDescription: "investments:timeDeposit.deleteDetailDescription",
  },
  tourKeyPrefix: "investments:timeDeposit",
  // TimeDeposit's regions exceed the five standard anchors — investment headline +
  // transaction table — so it overrides the whole tour list.
  tourSteps: (t) => [
    {
      element: '[data-testid="tour-overview"]',
      title: t("investments:timeDeposit.tour.overviewTitle"),
      description: t("investments:timeDeposit.tour.overviewBody"),
    },
    {
      element: '[data-testid="investment-headline"]',
      title: t("investments:timeDeposit.tour.headlineTitle"),
      description: t("investments:timeDeposit.tour.headlineBody"),
    },
    {
      element: '[data-testid="tour-actions"]',
      title: t("investments:timeDeposit.tour.actionsTitle"),
      description: t("investments:timeDeposit.tour.actionsBody"),
    },
    {
      element: '[data-testid="tour-details"]',
      title: t("investments:timeDeposit.tour.detailsTitle"),
      description: t("investments:timeDeposit.tour.detailsBody"),
    },
    {
      element: '[data-testid="tour-chart"]',
      title: t("investments:timeDeposit.tour.chartTitle"),
      description: t("investments:timeDeposit.tour.chartBody"),
    },
    {
      element: '[data-testid="tour-snapshots"]',
      title: t("investments:timeDeposit.tour.snapshotsTitle"),
      description: t("investments:timeDeposit.tour.snapshotsBody"),
    },
    {
      element: '[data-testid="tour-transactions"]',
      title: t("investments:timeDeposit.tour.transactionsTitle"),
      description: t("investments:timeDeposit.tour.transactionsBody"),
    },
  ],

  useEntity: useTimeDeposit,
  useDelete: useDeleteTimeDeposit,
  exportUrl: timeDepositExportUrl,
  getAsset: (entity) => entity.investment,

  renderHeaderBadge: (entity) => (
    <RiskProfileBadge profile={entity.investment.risk_profile} compact />
  ),

  useDetailContext: (assetId) => {
    const navigate = useNavigate();
    const { data: transactions } = useInvestmentTransactions(assetId);
    // Maturity flips the parent to 'matured' + upserts a close snapshot, so the
    // create hook carries the "time-deposits" detailKey to refresh the caches.
    const createTransactionMutation = useCreateInvestmentTransaction(assetId, "time-deposits");
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
      selectTimeDeposit: (id: string) => navigate(routes.timeDeposit(id)),
    };
  },

  // Bank name as the identity cluster at the top of the left column (ADR-0051
  // Phase B) — moved off the H1 subtitle.
  identityCluster: (entity) => <IdentityCluster lines={[entity.details.bank_name]} />,
  // Left column: interest rate + term — short values that read fine in the narrow
  // column. The date-range Period + at-maturity policy live in the wider middle
  // column (see renderHeadline) so they don't wrap onto two lines here.
  infoFields: (entity, _ctx, t) => {
    const { details } = entity;
    const fields: InfoField[] = [
      {
        label: t("investments:timeDeposit.interestRateLabel"),
        value: (
          <span className="tabular-nums">
            {t("investments:timeDeposit.interestRateValue", {
              rate: Number(details.interest_rate).toFixed(2),
            })}
          </span>
        ),
      },
      {
        label: t("investments:timeDeposit.termLabel"),
        value: (
          <span className="tabular-nums">
            {t("investments:timeDeposit.termValue", { months: details.term_months })}
          </span>
        ),
      },
    ];
    return fields;
  },

  // Middle column: risk + the money summary (cost = principal), plus the
  // date-range Period and at-maturity policy threaded in as extraFields — the
  // wider column fits the "1 Jan → 1 Jul 2026" range on one line. Principal is
  // not repeated; it is the headline's Total cost.
  renderHeadline: (entity, _ctx, snapshots, t) => {
    const latest = snapshots && snapshots.length > 0 ? snapshots[0] : null;
    const { details } = entity;
    const rolloverLabel = t(`investments:timeDeposit.rolloverPolicy.${details.rollover_policy}`);
    return (
      <InvestmentHeadline
        currency={entity.investment.native_currency}
        extraFields={[
          {
            label: t("investments:timeDeposit.periodLabel"),
            value: (
              <span className="tabular-nums">
                {t("investments:timeDeposit.periodValue", {
                  start: formatDate(details.placement_date),
                  end: formatDate(details.maturity_date),
                })}
              </span>
            ),
          },
          {
            label: t("investments:timeDeposit.atMaturityLabel"),
            value: rolloverLabel,
          },
        ]}
        latestValue={latest ? Number(latest.amount) : null}
        totalCost={Number(entity.details.principal)}
        status={entity.investment.status}
        terminatedAt={entity.investment.terminated_at}
      />
    );
  },

  // The post-maturity rollover callout: only present once a maturity txn has
  // actually rolled funds. Offers to spawn the successor deposit (prefilled) or
  // link one already hand-created (`LinkRolloverSuccessorDialog`, #65).
  renderBeforeDetails: (entity, ctx, t) => {
    const rollover = maturityRolloverPrefill(entity, ctx.transactions);
    if (!rollover) return null;
    return (
      <div
        data-testid="rollover-callout"
        className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-sky-200 bg-sky-50 px-4 py-3 text-sky-900"
      >
        <div className="flex items-start gap-3">
          <Repeat className="mt-0.5 size-5 shrink-0 text-sky-600" />
          <div className="text-sm">
            <p className="font-medium">{t("investments:timeDeposit.rollover.calloutTitle")}</p>
            <p className="text-sky-800">
              {t("investments:timeDeposit.rollover.calloutBody", {
                amount: formatCurrency(
                  rollover.rolledAmount.toString(),
                  entity.investment.native_currency,
                ),
              })}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <CreateTimeDepositDialog
            prefill={rollover.prefill}
            rolledFromInvestmentId={entity.investment.id}
            triggerLabel={t("investments:timeDeposit.rollover.calloutAction")}
          />
          <LinkRolloverSuccessorDialog sourceId={entity.investment.id} />
        </div>
      </div>
    );
  },

  // The rollover-chain card: the from/into links between successive deposits.
  // Present whenever this deposit is part of a chain; the navigation is descriptor
  // wiring via `ctx.selectTimeDeposit`, the core just renders the node.
  renderAfterDetails: (entity, ctx, t) => {
    if (!entity.rolled_from && !entity.rolled_to) return null;
    return (
      <Card data-testid="rollover-card">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Repeat className="size-4 text-muted-foreground" />
            {t("investments:timeDeposit.rolloverChain.title")}
          </CardTitle>
          <CardDescription>
            {t("investments:timeDeposit.rolloverChain.description")}
          </CardDescription>
        </CardHeader>
        <CardContent className="text-sm space-y-2">
          <div className="flex items-center gap-2">
            <ArrowUp className="size-4 shrink-0 text-muted-foreground" />
            <span className="text-muted-foreground">
              {t("investments:timeDeposit.rolloverChain.fromLabel")}
            </span>
            {entity.rolled_from ? (
              <button
                type="button"
                data-testid="rollover-from-link"
                className="font-medium text-sky-700 underline underline-offset-2 hover:text-sky-900"
                onClick={() => ctx.selectTimeDeposit(entity.rolled_from!.id)}
              >
                {entity.rolled_from.display_name}
              </button>
            ) : (
              <span className="text-muted-foreground">
                {t("investments:timeDeposit.rolloverChain.none")}
              </span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <ArrowDown className="size-4 shrink-0 text-muted-foreground" />
            <span className="text-muted-foreground">
              {t("investments:timeDeposit.rolloverChain.intoLabel")}
            </span>
            {entity.rolled_to ? (
              <button
                type="button"
                data-testid="rollover-into-link"
                className="font-medium text-sky-700 underline underline-offset-2 hover:text-sky-900"
                onClick={() => ctx.selectTimeDeposit(entity.rolled_to!.id)}
              >
                {entity.rolled_to.display_name}
              </button>
            ) : (
              <span className="text-muted-foreground">
                {t("investments:timeDeposit.rolloverChain.noneYet")}
              </span>
            )}
          </div>
        </CardContent>
      </Card>
    );
  },

  chartCostSeries: (entity, _ctx, snapshots) =>
    flatCostSeries(snapshots ?? [], Number(entity.details.principal)),

  snapshot: {
    useSectionRender: (assetId) => {
      const { t } = useTranslation(["investments"]);
      const { data: snapshots } = useInvestmentSnapshots(assetId);
      const createMutation = useCreateInvestmentSnapshot(assetId, "time-deposits");
      const updateMutation = useUpdateInvestmentSnapshot(assetId, "time-deposits");
      const deleteMutation = useDeleteInvestmentSnapshot(assetId, "time-deposits");
      const importMutation = useImportInvestmentSnapshots(assetId, "time-deposits");
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
        renderCard: (snapshot) => (
          <AccruedInterestSnapshotCard
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

    const txnSection: HistorySectionSpec<InvestmentTransaction> = {
      testId: "tour-transactions",
      title: t("investments:transactions.cardTitle"),
      description: t("investments:timeDeposit.transactionsDescription"),
      emptyText:
        allTxns.length === 0
          ? t("investments:timeDeposit.transactionsEmpty")
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
          quantityUnit=""
          updateMutation={ctx.updateTransactionMutation}
          deleteMutation={ctx.deleteTransactionMutation}
        />
      ),
      renderCard: (tx: InvestmentTransaction) => (
        <TransactionCard
          key={tx.id}
          transaction={tx}
          quantityUnit=""
          updateMutation={ctx.updateTransactionMutation}
          deleteMutation={ctx.deleteTransactionMutation}
        />
      ),
      pageSize: PAGE_SIZE,
      headerActions: active ? (
        <CreateMaturityTransactionDialog
          currency={currency}
          rolloverPolicy={entity.details.rollover_policy}
          placementDate={entity.details.placement_date?.slice(0, 10)}
          maturityDate={entity.details.maturity_date?.slice(0, 10)}
          mutation={ctx.createTransactionMutation}
        />
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
    };

    return [erasedSection(txnSection)];
  },

  renderEditDialog: (entity, props) => (
    <EditTimeDepositDialog key={entity.investment.id} timeDeposit={entity} {...props} />
  ),
};
