// The ownership column every list carries, investment and non-investment
// alike (#838): same label, same privacy rules (INV-PRESENTATION-03).
/* eslint-disable react-refresh/only-export-components */
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession, type Me } from "@/hooks/useSession";
import { ownershipLabel } from "@/lib/ownership";
import type { HouseholdMember } from "@/api/types";
import type { PositionExtraColumn } from "@/components/positionList/types";

export type OwnershipCtx = {
  members: HouseholdMember[] | undefined;
  currentUser: Me | null | undefined;
};

export type OwnershipCore = {
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
};

export function useOwnershipContext(): OwnershipCtx {
  const { data: members } = useHouseholdMembers();
  const { data: currentUser } = useSession();
  return { members, currentUser };
}

export function ownershipColumn<T>(
  entity: (item: T) => OwnershipCore,
): PositionExtraColumn<T, OwnershipCtx> {
  const label = (item: T, ctx: OwnershipCtx) => {
    const core = entity(item);
    return ownershipLabel(
      core.ownership_type,
      core.sole_owner_user_id,
      ctx.members,
      ctx.currentUser,
    );
  };
  return {
    id: "ownership",
    labelKey: "common:tableHeaders.ownership",
    slot: "ownership",
    render: label,
    sort: { key: "ownership", type: "text", value: label },
  };
}
