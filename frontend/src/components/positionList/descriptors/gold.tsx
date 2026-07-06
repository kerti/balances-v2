import { CreateGoldDialog } from "@/components/CreateGoldDialog";
import { EditGoldDialog } from "@/components/EditGoldDialog";
import { useGolds, useDeleteGold, useImportCreateGold } from "@/hooks/useInvestments";
import { investmentDescriptor } from "@/components/positionList/presets/investment";
import { formatGoldPurity } from "@/lib/gold";
import i18n from "@/i18n";
import type { GoldListItem } from "@/api/types";

// Gold, the investment cluster's tracer type (#332). Its only subtype column is
// form + purity; everything else — the risk badge, risk filter, activity cell,
// cost/P-L headline — comes from the investment preset.
export const goldDescriptor = investmentDescriptor<GoldListItem>({
  entityKey: "gold",
  testIdPrefix: "gold",
  i18nNamespaces: ["investments", "common", "errors"],
  keys: {
    listTitle: "investments:gold.listTitle",
    listSubtitle: "investments:gold.listSubtitle",
    emptyTitle: "investments:gold.emptyTitle",
    emptyBody: "investments:gold.emptyBody",
    noun: "investments:list.noun",
    nounPlural: "investments:list.nounPlural",
    valueLabel: "investments:gold.sortLatestValue",
    rowActions: "investments:gold.rowActions",
    deleteTitle: "investments:gold.deleteTitle",
  },
  useList: useGolds,
  useDelete: useDeleteGold,
  useImport: useImportCreateGold,
  entity: (item) => item.investment,
  getSnapshot: (item) => item.latest_snapshot,
  costBasis: (item) => Number(item.cost_basis),
  activity: (item) => ({
    count: item.transaction_count,
    lastDate: item.last_transaction_date,
  }),
  mainColumns: [
    {
      id: "purity",
      labelKey: "investments:gold.formPurityHeader",
      slot: "main",
      // Global i18n (not a `t` param): the extra-column render is
      // presentation-neutral and receives no translator; matches ownershipLabel.
      render: (item) => (
        <>
          <div>{i18n.t(`investments:gold.goldForms.${item.details.form}`)}</div>
          <div className="text-xs text-muted-foreground">
            {formatGoldPurity(item.details.purity)}
          </div>
        </>
      ),
    },
  ],
  deleteDescription: (item, t) =>
    t("investments:gold.deleteRowDescription", {
      name: item.investment.display_name,
    }),
  headlineTestId: "gold-total",
  renderCreateDialog: () => <CreateGoldDialog />,
  renderEditDialog: (item, props) => (
    <EditGoldDialog key={item.investment.id} gold={item} {...props} />
  ),
});
