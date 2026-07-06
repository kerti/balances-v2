import { describe, it, expect } from "vitest";
import { ownershipLabel } from "@/lib/ownership";
import type { HouseholdMember } from "@/api/types";
import type { Me } from "@/hooks/useSession";

// ownershipLabel only reads id/display_name/nickname off members and id off the
// current user, so fixtures carry just those (cast past the full wire types).
const member = (
  id: string,
  display_name: string,
  nickname: string | null = null,
): HouseholdMember => ({ id, display_name, nickname }) as HouseholdMember;

const me = (id: string): Me => ({ id }) as Me;

const alice = member("u-alice", "Alice");
const bob = member("u-bob", "Bob");
const members = [alice, bob];

// covers: INV-PRESENTATION-03
describe("ownershipLabel", () => {
  it('returns "Joint" for joint ownership regardless of other args', () => {
    expect(ownershipLabel("joint", null, undefined, undefined)).toBe("Joint");
    expect(ownershipLabel("joint", "u-alice", members, me("u-alice"))).toBe("Joint");
  });

  it("returns the owner display_name for sole ownership by another member", () => {
    expect(ownershipLabel("sole", "u-bob", members, me("u-alice"))).toBe("Bob");
  });

  it('suffixes "(you)" when the sole owner is the current user', () => {
    expect(ownershipLabel("sole", "u-alice", members, me("u-alice"))).toBe("Alice (you)");
  });

  it("prefers the owner nickname over display_name", () => {
    const withNick = [member("u-bob", "Robert Bobson", "Bob B")];
    expect(ownershipLabel("sole", "u-bob", withNick, me("u-alice"))).toBe("Bob B");
  });

  it('prefers the nickname even with the "(you)" suffix', () => {
    const withNick = [member("u-alice", "Alice Anderson", "Ali")];
    expect(ownershipLabel("sole", "u-alice", withNick, me("u-alice"))).toBe("Ali (you)");
  });

  it('does not suffix "(you)" when there is no current user', () => {
    expect(ownershipLabel("sole", "u-alice", members, null)).toBe("Alice");
    expect(ownershipLabel("sole", "u-alice", members, undefined)).toBe("Alice");
  });

  it('falls back to "Sole" while the member list is still loading', () => {
    expect(ownershipLabel("sole", "u-alice", undefined, me("u-alice"))).toBe("Sole");
  });

  it('falls back to "Sole" when the owner cannot be resolved (e.g. soft-deleted)', () => {
    expect(ownershipLabel("sole", "u-gone", members, me("u-alice"))).toBe("Sole");
    expect(ownershipLabel("sole", null, members, me("u-alice"))).toBe("Sole");
  });
});
