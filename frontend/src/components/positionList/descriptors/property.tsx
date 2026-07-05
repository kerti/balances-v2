import { CreatePropertyDialog } from "@/components/CreatePropertyDialog";
import { EditPropertyDialog } from "@/components/EditPropertyDialog";
import {
  useProperties,
  useDeleteProperty,
  useImportCreateProperty,
} from "@/hooks/useProperties";
import { nonInvestmentDescriptor } from "@/components/positionList/presets/nonInvestment";
import type { PropertyListItem } from "@/api/types";

// Property, on the non-investment preset (ADR-0043). property_type is a closed
// enum (house/apartment/land/commercial) translated against propertyTypes;
// address is free-form user text.
export const propertyDescriptor = nonInvestmentDescriptor<PropertyListItem>({
  entityKey: "property",
  testIdPrefix: "property",
  group: "assets",
  i18nNamespaces: ["assets", "common", "errors"],
  keys: {
    listTitle: "assets:property.listTitle",
    listSubtitle: "assets:property.listSubtitle",
    emptyTitle: "assets:property.emptyTitle",
    emptyBody: "assets:property.emptyBody",
    noun: "assets:property.noun",
    nounPlural: "assets:property.nounPlural",
    valueLabel: "assets:property.sortLatestValuation",
    rowActions: "assets:property.rowActions",
    deleteTitle: "assets:property.deleteTitle",
  },
  useList: useProperties,
  useDelete: useDeleteProperty,
  useImport: useImportCreateProperty,
  entity: (item) => item.asset,
  getSnapshot: (item) => item.latest_snapshot,
  getSecondary: (item, t) => {
    const typeLabel = t(
      `assets:property.propertyTypes.${item.details.property_type}`,
    );
    return [typeLabel, item.details.address].filter(Boolean).join(" · ") || "—";
  },
  deleteDescription: (item, t) =>
    t("assets:property.deleteRowDescription", {
      name: item.asset.display_name,
    }),
  headlineLabelKey: "assets:property.totalValue",
  headlineTestId: "properties-total",
  renderCreateDialog: () => <CreatePropertyDialog />,
  renderEditDialog: (item, props) => (
    <EditPropertyDialog
      key={item.asset.id}
      property={{ asset: item.asset, details: item.details }}
      {...props}
    />
  ),
});
