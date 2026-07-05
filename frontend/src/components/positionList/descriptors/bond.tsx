import { CreateBondDialog } from "@/components/CreateBondDialog";
import { EditBondDialog } from "@/components/EditBondDialog";
import {
  useBonds,
  useDeleteBond,
  useImportCreateBond,
} from "@/hooks/useInvestments";
import { investmentDescriptor } from "@/components/positionList/presets/investment";
import { maturityClass, maturityInfo } from "@/lib/maturity";
import { isActiveStatus } from "@/lib/lifecycle";
import i18n from "@/i18n";
import type { BondListItem, CouponFrequency } from "@/api/types";

const FREQUENCY_SHORT_KEY: Record<CouponFrequency, string> = {
  monthly: "investments:bond.couponFrequency.monthly_short",
  quarterly: "investments:bond.couponFrequency.quarterly_short",
  semi_annual: "investments:bond.couponFrequency.semi_annual_short",
  annual: "investments:bond.couponFrequency.annual_short",
};

// Bond, on the investment preset (ADR-0043). Subtype column = series code +
// type/issuer/rate/frequency meta + a maturity countdown (hidden once
// terminated).
export const bondDescriptor = investmentDescriptor<BondListItem>({
  entityKey: "bond",
  testIdPrefix: "bond",
  i18nNamespaces: ["investments", "common", "errors"],
  keys: {
    listTitle: "investments:bond.listTitle",
    listSubtitle: "investments:bond.listSubtitle",
    emptyTitle: "investments:bond.emptyTitle",
    emptyBody: "investments:bond.emptyBody",
    noun: "investments:list.noun",
    nounPlural: "investments:list.nounPlural",
    valueLabel: "investments:bond.sortLatestValue",
    rowActions: "investments:bond.rowActions",
    deleteTitle: "investments:bond.deleteTitle",
  },
  useList: useBonds,
  useDelete: useDeleteBond,
  useImport: useImportCreateBond,
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
      labelKey: "investments:bond.identityHeader",
      slot: "main",
      render: (item) => {
        const terminated = !isActiveStatus(item.investment.status);
        const mInfo = maturityInfo(item.details.maturity_date);
        const bondType = i18n.t(
          item.details.bond_type === "govt_primary"
            ? "investments:bond.bondType.govt_primary_short"
            : "investments:bond.bondType.secondary_market_short",
        );
        return (
          <>
            {item.details.series_code ? (
              <div className="font-mono text-sm">
                {item.details.series_code}
              </div>
            ) : (
              <div className="text-sm text-muted-foreground">{"—"}</div>
            )}
            <div className="text-xs text-muted-foreground">
              {i18n.t("investments:bond.rowMeta", {
                type: bondType,
                issuer: item.details.issuer,
                rate: Number(item.details.coupon_rate).toFixed(2),
                frequency: i18n.t(
                  FREQUENCY_SHORT_KEY[item.details.coupon_frequency],
                ),
              })}
            </div>
            {!terminated && (
              <div className={`text-xs ${maturityClass(mInfo.state)}`}>
                {mInfo.label}
              </div>
            )}
          </>
        );
      },
    },
  ],
  deleteDescription: (item, t) =>
    t("investments:bond.deleteRowDescription", {
      name: item.investment.display_name,
    }),
  headlineTestId: "bonds-total",
  renderCreateDialog: () => <CreateBondDialog />,
  renderEditDialog: (item, props) => (
    <EditBondDialog key={item.investment.id} bond={item} {...props} />
  ),
});
