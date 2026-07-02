// Centralised route paths + builders. The router (App.tsx), the sidebar, every
// list row's onSelect, and every detail's back-link reference these instead of
// literal path strings — so a renamed path is a single edit and a mistyped link
// is a TypeScript error rather than a runtime 404. This is the deliberate
// stand-in for the compile-time link checking a type-safe router would give;
// see ADR-0025 for why we chose React Router + this convention over TanStack
// Router.

export const routes = {
  dashboard: "/",

  // Post-auth onboarding gate (ADR-0038). Rendered outside the authed router
  // (App.tsx) — the holder has a handshake cookie but no session yet.
  onboarding: "/onboarding",

  // Local-invite accept (ADR-0039/#281). Where an invite link lands a local
  // invitee: they set a password and the account is created bound to the
  // invited email. Rendered outside the authed router — the holder has neither a
  // session nor a handshake, only the single-use token in the URL.
  accept: "/accept",

  // Emailed password reset (ADR-0039/#282). forgotPassword is the request form
  // (reached from the sign-in screen's "Forgot password?" link); resetPassword
  // is where the emailed link lands, carrying the single-use token in the URL.
  // Both render outside the authed router — the holder has no session.
  forgotPassword: "/forgot-password",
  resetPassword: "/reset",

  // Post-erasure landing (ADR-0040/#300). Where the founder lands right after
  // deleting their household — the erase commit clears the session cookie
  // instead of re-issuing one (there's no household left to sign back into),
  // so this renders outside the authed router like the other pre-session
  // screens. Deliberately not the sign-in screen: signing back in would just
  // re-trigger onboarding as if nothing happened.
  erased: "/erased",

  // Assets — home + three subtype lists, each with a detail under it.
  assets: "/assets",
  bankAccounts: "/assets/bank-accounts",
  bankAccount: (id: string) => `/assets/bank-accounts/${id}`,
  properties: "/assets/properties",
  property: (id: string) => `/assets/properties/${id}`,
  vehicles: "/assets/vehicles",
  vehicle: (id: string) => `/assets/vehicles/${id}`,

  // Liabilities — home + two subtype lists. Detail nests under the subtype
  // (`/liabilities/personal/:id`) so the dynamic `:id` never overlaps the
  // literal `personal`/`institutional` segments. ADR-0025.
  liabilities: "/liabilities",
  liabilitiesPersonal: "/liabilities/personal",
  liabilitiesInstitutional: "/liabilities/institutional",
  liability: (subtype: "personal" | "institutional", id: string) =>
    `/liabilities/${subtype}/${id}`,

  // Receivables — flat group: the list is the root path, no home page.
  receivables: "/receivables",
  receivable: (id: string) => `/receivables/${id}`,

  // Investments — home + five subtype lists, each with a detail under it.
  investments: "/investments",
  stocks: "/investments/stocks",
  stock: (id: string) => `/investments/stocks/${id}`,
  mutualFunds: "/investments/mutual-funds",
  mutualFund: (id: string) => `/investments/mutual-funds/${id}`,
  bonds: "/investments/bonds",
  bond: (id: string) => `/investments/bonds/${id}`,
  timeDeposits: "/investments/time-deposits",
  timeDeposit: (id: string) => `/investments/time-deposits/${id}`,
  gold: "/investments/gold",
  goldItem: (id: string) => `/investments/gold/${id}`,

  // Income — flow event, not a position group: a flat list at its own path.
  income: "/income",

  // Tags — the cross-group breakdown report; flat, no detail pages.
  tags: "/tags",

  settings: "/settings",
} as const;

// positionDetail resolves a position's detail-page path from the (group,
// subtype) pair the report carries for stale positions (#50). Returns null for
// any pair without a detail page (none today) so callers render a plain label
// instead of a dead link. The subtype strings are the engine's wire values
// (matching the DB subtype columns), not display labels.
export function positionDetail(
  group: string,
  subtype: string,
  id: string,
): string | null {
  switch (group) {
    case "asset":
      switch (subtype) {
        case "bank_account":
          return routes.bankAccount(id);
        case "property":
          return routes.property(id);
        case "vehicle":
          return routes.vehicle(id);
      }
      return null;
    case "liability":
      if (subtype === "personal" || subtype === "institutional") {
        return routes.liability(subtype, id);
      }
      return null;
    case "receivable":
      return routes.receivable(id);
    case "investment":
      switch (subtype) {
        case "stock":
          return routes.stock(id);
        case "mutual_fund":
          return routes.mutualFund(id);
        case "bond":
          return routes.bond(id);
        case "time_deposit":
          return routes.timeDeposit(id);
        case "gold":
          return routes.goldItem(id);
      }
      return null;
  }
  return null;
}
