import { CreateStockDialog } from "@/components/CreateStockDialog";
import { EditStockDialog } from "@/components/EditStockDialog";
import {
  useStocks,
  useDeleteStock,
  useImportCreateStock,
} from "@/hooks/useInvestments";
import { investmentDescriptor } from "@/components/positionList/presets/investment";
import type { StockListItem } from "@/api/types";

// Stock, on the investment preset (ADR-0043). Subtype column = ticker +
// exchange.
export const stockDescriptor = investmentDescriptor<StockListItem>({
  entityKey: "stock",
  testIdPrefix: "stock",
  i18nNamespaces: ["investments", "common", "errors"],
  keys: {
    listTitle: "investments:stock.listTitle",
    listSubtitle: "investments:stock.listSubtitle",
    emptyTitle: "investments:stock.emptyTitle",
    emptyBody: "investments:stock.emptyBody",
    noun: "investments:list.noun",
    nounPlural: "investments:list.nounPlural",
    valueLabel: "investments:stock.sortLatestValue",
    rowActions: "investments:stock.rowActions",
    deleteTitle: "investments:stock.deleteTitle",
  },
  useList: useStocks,
  useDelete: useDeleteStock,
  useImport: useImportCreateStock,
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
      labelKey: "investments:stock.tickerHeader",
      slot: "main",
      render: (item) => (
        <>
          <div className="font-mono text-sm">{item.details.ticker}</div>
          <div className="text-xs text-muted-foreground">
            {item.details.exchange}
          </div>
        </>
      ),
    },
  ],
  deleteDescription: (item, t) =>
    t("investments:stock.deleteRowDescription", {
      name: item.investment.display_name,
    }),
  headlineTestId: "stocks-total",
  renderCreateDialog: () => <CreateStockDialog />,
  renderEditDialog: (item, props) => (
    <EditStockDialog key={item.investment.id} stock={item} {...props} />
  ),
});
