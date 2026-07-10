import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, ApiError, isEnvelope, type ErrorEnvelope } from "@/api/client";

// Bulk monthly-entry (ADR-0046) generalised across the amount-only groups
// (Asset/Liability/Receivable, #421→#422). The wire shapes are per-group only
// in the position-id field name — `asset_id` / `liability_id` / `receivable_id`
// — so a small data-layer config carries the API base + that field name, and
// these hooks normalise every group to a single `position_id`-keyed shape the
// EntryScreen renders uniformly.
export type EntryGroup =
  "assets" | "liabilities" | "receivables" | "investments" | "investments-accrued";

// EntryDataConfig is the data-layer half of a group's entry config: where its
// endpoints live, the wire name of its position id, and the query keys a
// committed save invalidates so carried-forward net worth refreshes.
export type EntryDataConfig = {
  group: EntryGroup;
  apiBase: string; // e.g. "/api/assets/snapshots"
  idField: string; // e.g. "asset_id"
  invalidateKeys: readonly (readonly string[])[];
};

// One row of the bulk monthly-entry list, normalised: an eligible position with
// its carry-forward prefill. carried_from is null for a position with no
// snapshot at or before the target month. `subtype` is "" for a flat group
// (receivables). `position_id` abstracts the per-group id field.
//
// The prefill fields are shape-specific (ADR-0022): amount-only groups populate
// `prefill_amount`; the qty×price group (#423) populates `prefill_quantity` +
// `prefill_price` instead; the accrued group (#424) populates `prefill_amount`
// (the total value) + `prefill_accrued_interest`. An `EntryShape` reads
// whichever fields its group uses; the unused ones stay null. `coupon_disposition`
// is carried only for the accrued group (a bond's, driving its per-row accrued
// default); null for every other group and for time deposits.
export type EntryRow = {
  position_id: string;
  display_name: string;
  currency: string;
  subtype: string;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  prefill_amount: string | null;
  prefill_quantity: string | null;
  prefill_price: string | null;
  prefill_accrued_interest: string | null;
  coupon_disposition: string | null;
  carried_from: string | null;
};

export type EntryList = {
  year_month: string;
  rows: EntryRow[];
};

// One dirty row the user changed — the client sends only these. `fields` holds
// the shape's wire columns keyed by name (amount-only: `{ amount }`; qty×price:
// `{ quantity, price_per_unit }`), spread flat into the save body alongside the
// group's id field and currency.
export type BulkSnapshotRow = {
  position_id: string;
  currency: string;
  fields: Record<string, string>;
};

export type BulkSaveArgs = {
  year_month: string;
  as_of_date: string;
  rows: BulkSnapshotRow[];
};

// The 422 per-row error (ADR-0046), normalised to `position_id`.
export type BulkRowError = { position_id: string; code: string };

// A save either commits (ok) or is rejected with per-row errors (a 422 that is
// data, not a thrown error — mirrors postCreateImport's 422 handling).
export type BulkSaveResult = { ok: true; written: number } | { ok: false; errors: BulkRowError[] };

// The raw per-group wire row — the position id lives under a group-specific
// key, everything else is shared. Read off `Record` so the id key can be
// resolved dynamically from the config.
type RawEntryRow = Record<string, unknown> & {
  display_name: string;
  currency: string;
  subtype?: string;
  ownership_type: "sole" | "joint";
  sole_owner_user_id: string | null;
  prefill_amount?: string | null;
  prefill_quantity?: string | null;
  prefill_price?: string | null;
  prefill_accrued_interest?: string | null;
  coupon_disposition?: string | null;
  carried_from: string | null;
};

// useEntryList fetches the eligible-positions-with-prefill list for a target
// month and normalises each row's group-specific id field to `position_id`.
// Disabled until a month is chosen.
export function useEntryList(cfg: EntryDataConfig, yearMonth: string | null) {
  return useQuery({
    queryKey: [cfg.group, "entry", yearMonth],
    queryFn: async (): Promise<EntryList> => {
      const raw = await api<{ year_month: string; rows: RawEntryRow[] }>(
        `${cfg.apiBase}/entry?year_month=${yearMonth}`,
      );
      return {
        year_month: raw.year_month,
        rows: raw.rows.map((r) => ({
          position_id: String(r[cfg.idField]),
          display_name: r.display_name,
          currency: r.currency,
          subtype: r.subtype ?? "",
          ownership_type: r.ownership_type,
          sole_owner_user_id: r.sole_owner_user_id,
          prefill_amount: r.prefill_amount ?? null,
          prefill_quantity: r.prefill_quantity ?? null,
          prefill_price: r.prefill_price ?? null,
          prefill_accrued_interest: r.prefill_accrued_interest ?? null,
          coupon_disposition: r.coupon_disposition ?? null,
          carried_from: r.carried_from,
        })),
      };
    },
    enabled: !!yearMonth,
    staleTime: 0,
  });
}

// useBulkSaveSnapshots posts a whole batch for a group. A 422 (per-row
// rejections) is returned as data — `{ ok: false }` — not thrown, since the
// envelope-filtering `api()` wrapper would drop the non-envelope body; every
// other non-2xx throws ApiError as usual. The request maps the normalised
// `position_id` rows back to the group's wire id field, and the 422 body back
// to `position_id`. On a committed save it invalidates the group's lists +
// reports so carried-forward net worth refreshes.
export function useBulkSaveSnapshots(cfg: EntryDataConfig) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (args: BulkSaveArgs): Promise<BulkSaveResult> => {
      const body = {
        year_month: args.year_month,
        as_of_date: args.as_of_date,
        rows: args.rows.map((r) => ({
          [cfg.idField]: r.position_id,
          currency: r.currency,
          ...r.fields,
        })),
      };
      const res = await fetch(`${cfg.apiBase}/bulk`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (res.status === 422) {
        const parsed = (await res.json()) as {
          errors: Array<Record<string, unknown> & { code: string }>;
        };
        return {
          ok: false,
          errors: parsed.errors.map((e) => ({ position_id: String(e[cfg.idField]), code: e.code })),
        };
      }
      if (!res.ok) {
        let errBody: ErrorEnvelope | undefined;
        try {
          const parsed = await res.json();
          errBody = isEnvelope(parsed) ? parsed : undefined;
        } catch {
          errBody = undefined;
        }
        throw new ApiError(
          res.status,
          res.statusText || `bulk save failed (${res.status})`,
          errBody,
        );
      }
      const ok = (await res.json()) as { written: number };
      return { ok: true, written: ok.written };
    },
    onSuccess: (result) => {
      if (!result.ok) return;
      for (const key of cfg.invalidateKeys) qc.invalidateQueries({ queryKey: [...key] });
      qc.invalidateQueries({ queryKey: ["reports"] });
    },
  });
}
