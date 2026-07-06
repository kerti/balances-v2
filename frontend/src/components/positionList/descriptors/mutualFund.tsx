import { CreateMutualFundDialog } from "@/components/CreateMutualFundDialog";
import { EditMutualFundDialog } from "@/components/EditMutualFundDialog";
import {
  useMutualFunds,
  useDeleteMutualFund,
  useImportCreateMutualFund,
} from "@/hooks/useInvestments";
import { investmentDescriptor } from "@/components/positionList/presets/investment";
import i18n from "@/i18n";
import type { MutualFundListItem } from "@/api/types";

// Mutual fund, on the investment preset (ADR-0043). Subtype column = fund code +
// manager; a fund-type chip rides beside the name as a title accessory.
export const mutualFundDescriptor = investmentDescriptor<MutualFundListItem>({
  entityKey: "mutualFund",
  testIdPrefix: "mutual-fund",
  i18nNamespaces: ["investments", "common", "errors"],
  keys: {
    listTitle: "investments:mutualFund.listTitle",
    listSubtitle: "investments:mutualFund.listSubtitle",
    emptyTitle: "investments:mutualFund.emptyTitle",
    emptyBody: "investments:mutualFund.emptyBody",
    noun: "investments:list.noun",
    nounPlural: "investments:list.nounPlural",
    valueLabel: "investments:mutualFund.sortLatestValue",
    rowActions: "investments:mutualFund.rowActions",
    deleteTitle: "investments:mutualFund.deleteTitle",
  },
  useList: useMutualFunds,
  useDelete: useDeleteMutualFund,
  useImport: useImportCreateMutualFund,
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
      labelKey: "investments:mutualFund.fundCodeHeader",
      slot: "main",
      render: (item) => (
        <>
          <div className="font-mono text-sm">{item.details.fund_code}</div>
          {item.details.fund_manager && (
            <div className="text-xs text-muted-foreground">{item.details.fund_manager}</div>
          )}
        </>
      ),
    },
  ],
  titleAccessory: (item) => (
    <span
      className="rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground"
      data-testid="mf-fund-type"
    >
      {i18n.t(`investments:mutualFund.fundType.short.${item.details.fund_type}`)}
    </span>
  ),
  deleteDescription: (item, t) =>
    t("investments:mutualFund.deleteRowDescription", {
      name: item.investment.display_name,
    }),
  headlineTestId: "mutual-funds-total",
  renderCreateDialog: () => <CreateMutualFundDialog />,
  renderEditDialog: (item, props) => (
    <EditMutualFundDialog key={item.investment.id} mutualFund={item} {...props} />
  ),
});
