import type { HouseholdMember } from "@/api/types";
import type { Me } from "@/hooks/useSession";
import i18n from "@/i18n";
import { preferredName } from "@/lib/names";

// Resolves the user-facing ownership label for a position-shaped row.
// Joint → "Joint". Sole → owner's preferred name (nickname ?? display_name,
// with "(you)" suffix when the owner is the current user). Falls back to
// "Sole" when the member list is still loading or the owner can't be
// resolved (e.g. soft-deleted user). All literal strings route through
// i18next so the label localises with the rest of the UI (ADR-0026).
export function ownershipLabel(
  ownershipType: "sole" | "joint",
  soleOwnerUserID: string | null,
  members: HouseholdMember[] | undefined,
  currentUser: Me | null | undefined,
): string {
  if (ownershipType === "joint") {
    return i18n.t("common:ownership.joint", { defaultValue: "Joint" });
  }
  const owner = (members ?? []).find((m) => m.id === soleOwnerUserID);
  if (!owner) return i18n.t("common:ownership.sole", { defaultValue: "Sole" });
  if (currentUser && owner.id === currentUser.id) {
    const youSuffix = i18n.t("common:ownership.youSuffix", {
      defaultValue: " (you)",
    });
    return `${preferredName(owner)}${youSuffix}`;
  }
  return preferredName(owner);
}
