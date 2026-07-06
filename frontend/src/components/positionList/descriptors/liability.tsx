import { CreateLiabilityDialog } from "@/components/CreateLiabilityDialog";
import { EditLiabilityDialog } from "@/components/EditLiabilityDialog";
import {
  useLiabilities,
  useDeleteLiability,
  useImportCreateLiability,
} from "@/hooks/useLiabilities";
import {
  nonInvestmentDescriptor,
  type OwnershipCtx,
} from "@/components/positionList/presets/nonInvestment";
import type { PositionListDescriptor } from "@/components/positionList/types";
import type { LiabilityListItem } from "@/api/types";

// Liability, on the non-investment preset (ADR-0043). Unlike its siblings the
// list is subtype-scoped (personal vs institutional) — same table, different
// list hook argument + subtype-interpolated title/empty copy — so it's a
// factory producing one descriptor per subtype.
type LiabilitySubtype = "personal" | "institutional";

export function liabilityDescriptor(
  subtype: LiabilitySubtype,
): PositionListDescriptor<LiabilityListItem, OwnershipCtx> {
  return nonInvestmentDescriptor<LiabilityListItem>({
    entityKey: `liability-${subtype}`,
    testIdPrefix: "liability",
    group: "liabilities",
    i18nNamespaces: ["liabilities", "common", "errors"],
    keys: {
      listTitle: `liabilities:screens.${subtype}.title`,
      listSubtitle: `liabilities:screens.${subtype}.description`,
      emptyTitle: "liabilities:emptyTitle",
      emptyBody: "liabilities:emptyBody",
      noun: "liabilities:noun",
      nounPlural: "liabilities:nounPlural",
      valueLabel: "liabilities:sortLatestBalance",
      rowActions: "liabilities:rowActions",
      deleteTitle: "liabilities:deleteTitle",
    },
    copyArgs: (t) => ({
      subtype: t(`liabilities:subtypes.${subtype}`).toLowerCase(),
    }),
    useList: () => useLiabilities(subtype),
    useDelete: useDeleteLiability,
    useImport: useImportCreateLiability,
    entity: (item) => item.liability,
    getSnapshot: (item) => item.latest_snapshot,
    getSecondary: (item) => item.liability.counterparty_name,
    deleteDescription: (item, t) =>
      t("liabilities:deleteRowDescription", {
        name: item.liability.display_name,
      }),
    headlineLabelKey: "liabilities:totalOwed",
    headlineTestId: "liabilities-total",
    renderCreateDialog: () => <CreateLiabilityDialog defaultSubtype={subtype} />,
    renderEditDialog: (item, props) => (
      <EditLiabilityDialog key={item.liability.id} liability={item.liability} {...props} />
    ),
  });
}

// Materialised once per subtype: descriptors are static, so the route wiring
// passes these constants rather than rebuilding on every render.
export const liabilityPersonalDescriptor = liabilityDescriptor("personal");
export const liabilityInstitutionalDescriptor = liabilityDescriptor("institutional");
