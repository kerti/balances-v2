import { EntryScreen } from "@/components/entry/EntryScreen";
import { assetEntryConfig } from "@/components/entry/groups";

// The Asset group's bulk monthly-entry view (ADR-0046, #421). A thin wrapper
// over the shared EntryScreen with the Asset config — kept as a named component
// so the route wiring and the #421 tests/testids are unchanged now that #422
// generalised the screen across the amount-only groups.
export function AssetEntryScreen() {
  return <EntryScreen config={assetEntryConfig} />;
}
