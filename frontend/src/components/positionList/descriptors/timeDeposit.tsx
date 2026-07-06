import { CreateTimeDepositDialog } from "@/components/CreateTimeDepositDialog";
import { EditTimeDepositDialog } from "@/components/EditTimeDepositDialog";
import {
  useTimeDeposits,
  useDeleteTimeDeposit,
  useImportCreateTimeDeposit,
} from "@/hooks/useInvestments";
import { investmentDescriptor } from "@/components/positionList/presets/investment";
import { maturityClass, maturityInfo } from "@/lib/maturity";
import { isActiveStatus } from "@/lib/lifecycle";
import i18n from "@/i18n";
import type { TimeDepositListItem } from "@/api/types";

// Time deposit, on the investment preset (ADR-0043). Subtype column = bank +
// rate/term meta + a maturity countdown (hidden once terminated).
export const timeDepositDescriptor = investmentDescriptor<TimeDepositListItem>({
  entityKey: "timeDeposit",
  testIdPrefix: "time-deposit",
  i18nNamespaces: ["investments", "common", "errors"],
  keys: {
    listTitle: "investments:timeDeposit.listTitle",
    listSubtitle: "investments:timeDeposit.listSubtitle",
    emptyTitle: "investments:timeDeposit.emptyTitle",
    emptyBody: "investments:timeDeposit.emptyBody",
    noun: "investments:list.noun",
    nounPlural: "investments:list.nounPlural",
    valueLabel: "investments:timeDeposit.sortLatestValue",
    rowActions: "investments:timeDeposit.rowActions",
    deleteTitle: "investments:timeDeposit.deleteTitle",
  },
  useList: useTimeDeposits,
  useDelete: useDeleteTimeDeposit,
  useImport: useImportCreateTimeDeposit,
  entity: (item) => item.investment,
  getSnapshot: (item) => item.latest_snapshot,
  costBasis: (item) => Number(item.cost_basis),
  activity: (item) => ({
    count: item.transaction_count,
    lastDate: item.last_transaction_date,
  }),
  mainColumns: [
    {
      id: "identity",
      labelKey: "investments:timeDeposit.identityHeader",
      slot: "main",
      render: (item) => {
        const terminated = !isActiveStatus(item.investment.status);
        const mInfo = maturityInfo(item.details.maturity_date);
        return (
          <>
            <div className="text-sm">{item.details.bank_name}</div>
            <div className="text-xs text-muted-foreground">
              {i18n.t("investments:timeDeposit.rowMeta", {
                rate: Number(item.details.interest_rate).toFixed(2),
                months: item.details.term_months,
              })}
            </div>
            {!terminated && (
              <div className={`text-xs ${maturityClass(mInfo.state)}`}>{mInfo.label}</div>
            )}
          </>
        );
      },
    },
  ],
  deleteDescription: (item, t) =>
    t("investments:timeDeposit.deleteRowDescription", {
      name: item.investment.display_name,
    }),
  headlineTestId: "time-deposits-total",
  renderCreateDialog: () => <CreateTimeDepositDialog />,
  renderEditDialog: (item, props) => (
    <EditTimeDepositDialog key={item.investment.id} timeDeposit={item} {...props} />
  ),
});
