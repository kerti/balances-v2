import { useState } from "react";
import { useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import { Landmark, Home, Car, Wallet, RotateCcw, type LucideIcon } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { thisYearMonth, monthEndDateCapped, monthStartDate } from "@/lib/dateLimits";
import { formatYearMonth } from "@/lib/format";
import { errorMessage } from "@/lib/errorMessage";
import { ownershipLabel } from "@/lib/ownership";
import { routes } from "@/lib/routes";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import {
  useAssetEntryList,
  useBulkSaveAssetSnapshots,
  type AssetEntryRow,
  type BulkSnapshotRow,
} from "@/hooks/useBulkEntry";

// Asset subtypes, in the order the entry view groups them, each with its icon
// and the i18n key (camelCase) for its section label — reusing AssetsHome's
// category labels (assets:home.categoryLabel.*) so the app reads consistently.
const SUBTYPE_META: Record<string, { icon: LucideIcon; labelKey: string }> = {
  bank_account: { icon: Landmark, labelKey: "bankAccount" },
  property: { icon: Home, labelKey: "property" },
  vehicle: { icon: Car, labelKey: "vehicle" },
};
const SUBTYPE_ORDER = ["bank_account", "property", "vehicle"];

// AssetEntryScreen is the Asset group's bulk monthly-entry view (ADR-0046):
// one screen listing every position eligible for a chosen month, grouped by
// type, each with its last value carried forward, a batch-level "when" control,
// and one Save. Only rows the user actually changed are sent (dirty-only);
// untouched positions ride the carry-forward rule.
export function AssetEntryScreen() {
  const { t } = useTranslation(["common", "assets"]);
  const navigate = useNavigate();

  const [yearMonth, setYearMonth] = useState(thisYearMonth());
  // as_of_date must fall within the target month (backend CHECK), so it seeds
  // to the end of that month (capped at today) rather than the single-snapshot
  // carryover_date_mode date, which does not generalise to an arbitrarily
  // chosen target month.
  const [asOfDate, setAsOfDate] = useState(monthEndDateCapped(thisYearMonth()));
  const [edits, setEdits] = useState<Record<string, string>>({});
  const [prevYearMonth, setPrevYearMonth] = useState(yearMonth);

  const list = useAssetEntryList(yearMonth);
  const save = useBulkSaveAssetSnapshots();
  const { data: members } = useHouseholdMembers();
  const { data: me } = useSession();

  // Guarded setState-during-render (no useEffect — lint bans setState-in-effect,
  // ADR-0041 follow-up): when the month changes, re-seed the as-of default,
  // clear edits, and drop any stale save error.
  if (prevYearMonth !== yearMonth) {
    setPrevYearMonth(yearMonth);
    setAsOfDate(monthEndDateCapped(yearMonth));
    setEdits({});
    save.reset();
  }

  const rows = list.data?.rows ?? [];

  // Per-row rejections from a 422 (ADR-0046) come back as data, keyed by asset.
  const rowErrors: Record<string, string> = {};
  if (save.data && !save.data.ok) {
    for (const e of save.data.errors) rowErrors[e.asset_id] = e.code;
  }
  // A thrown error is any non-2xx that isn't the per-row 422.
  const hasEnvelopeError = save.isError;

  function valueFor(row: AssetEntryRow): string {
    return edits[row.asset_id] ?? row.prefill_amount ?? "";
  }
  // A row is dirty when the user typed a non-empty value that differs from the
  // carried-forward prefill.
  function isDirty(row: AssetEntryRow): boolean {
    const v = edits[row.asset_id];
    if (v === undefined) return false;
    const trimmed = v.trim();
    return trimmed !== "" && trimmed !== (row.prefill_amount ?? "");
  }

  const dirtyRows: BulkSnapshotRow[] = rows.filter(isDirty).map((r) => ({
    asset_id: r.asset_id,
    amount: (edits[r.asset_id] ?? "").trim(),
    currency: r.currency,
  }));

  // Group eligible rows by subtype (rows arrive pre-ordered by subtype then
  // name), then present the known subtypes in SUBTYPE_ORDER, any unknown last.
  const grouped = new Map<string, AssetEntryRow[]>();
  for (const r of rows) {
    const g = grouped.get(r.subtype) ?? [];
    g.push(r);
    grouped.set(r.subtype, g);
  }
  const orderedSubtypes = [
    ...SUBTYPE_ORDER.filter((s) => grouped.has(s)),
    ...[...grouped.keys()].filter((s) => !SUBTYPE_ORDER.includes(s)),
  ];

  // resetRow drops the user's override so the field falls back to its
  // carried-forward prefill (empty for a position with no history).
  function resetRow(assetID: string) {
    setEdits((prev) => {
      const next = { ...prev };
      delete next[assetID];
      return next;
    });
  }

  function renderRow(row: AssetEntryRow) {
    const dirty = isDirty(row);
    return (
      <li
        key={row.asset_id}
        className="flex items-center gap-3 py-2"
        data-testid={`asset-entry-row-${row.asset_id}`}
      >
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
            {dirty && (
              <span
                className="size-1.5 shrink-0 rounded-full bg-amber-500"
                data-testid={`asset-entry-dirty-${row.asset_id}`}
                aria-hidden
              />
            )}
            <span className="truncate font-medium">{row.display_name}</span>
          </div>
          <div className="text-xs text-muted-foreground">
            {ownershipLabel(row.ownership_type, row.sole_owner_user_id, members, me)}
            {" · "}
            {row.carried_from === yearMonth ? (
              // A snapshot already exists for the chosen month — the prefill IS
              // this month's value, so editing it overwrites (upsert). Warn.
              <span
                className="text-amber-600"
                data-testid={`asset-entry-overwrite-${row.asset_id}`}
              >
                {t("bulkEntry.overwritesThisMonth")}
              </span>
            ) : row.carried_from ? (
              t("bulkEntry.carriedFrom", {
                month: formatYearMonth(`${row.carried_from}-01T00:00:00Z`),
              })
            ) : (
              t("bulkEntry.noHistory")
            )}
          </div>
          {rowErrors[row.asset_id] && (
            <div
              className="text-xs text-destructive"
              data-testid={`asset-entry-error-${row.asset_id}`}
            >
              {t("bulkEntry.rowError")}
            </div>
          )}
        </div>
        <span className="text-xs text-muted-foreground">{row.currency}</span>
        <Input
          className={`w-36${dirty ? " border-amber-500 ring-1 ring-amber-500" : ""}`}
          inputMode="decimal"
          value={valueFor(row)}
          onChange={(e) => setEdits({ ...edits, [row.asset_id]: e.target.value })}
          data-testid={`asset-entry-amount-${row.asset_id}`}
        />
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className={`size-8 shrink-0${dirty ? "" : " invisible"}`}
          onClick={() => resetRow(row.asset_id)}
          aria-label={t("bulkEntry.undo")}
          title={t("bulkEntry.undo")}
          disabled={!dirty}
          data-testid={`asset-entry-undo-${row.asset_id}`}
        >
          <RotateCcw className="size-4" />
        </Button>
      </li>
    );
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (dirtyRows.length === 0) return;
    save.mutate(
      { year_month: yearMonth, as_of_date: asOfDate, rows: dirtyRows },
      { onSuccess: (result) => result.ok && navigate(routes.assets) },
    );
  }

  return (
    <div className="mx-auto max-w-2xl p-4">
      <Button
        variant="ghost"
        size="sm"
        onClick={() => navigate(routes.assets)}
        className="-ml-2 mb-1"
        data-testid="asset-entry-back"
      >
        {t("common:actions.back")}
      </Button>
      <Card>
        <CardHeader>
          <CardTitle>{t("bulkEntry.title")}</CardTitle>
          <CardDescription>{t("bulkEntry.description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="space-y-4">
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-2">
                <Label htmlFor="bulk-year-month">{t("fields.month")}</Label>
                <Input
                  id="bulk-year-month"
                  type="month"
                  max={thisYearMonth()}
                  value={yearMonth}
                  onChange={(e) => setYearMonth(e.target.value)}
                  data-testid="asset-entry-month"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="bulk-as-of">{t("fields.statementDate")}</Label>
                <Input
                  id="bulk-as-of"
                  type="date"
                  min={monthStartDate(yearMonth)}
                  max={monthEndDateCapped(yearMonth)}
                  value={asOfDate}
                  onChange={(e) => setAsOfDate(e.target.value)}
                  data-testid="asset-entry-asof"
                />
              </div>
            </div>

            {list.isPending ? (
              <p className="text-sm text-muted-foreground">{t("loading")}</p>
            ) : rows.length === 0 ? (
              <p className="text-sm text-muted-foreground" data-testid="asset-entry-empty">
                {t("bulkEntry.empty")}
              </p>
            ) : (
              <div className="space-y-4">
                {orderedSubtypes.map((subtype) => {
                  const meta = SUBTYPE_META[subtype];
                  const Icon = meta?.icon ?? Wallet;
                  return (
                    <div key={subtype} data-testid={`asset-entry-group-${subtype}`}>
                      <div className="mb-1 flex items-center gap-2 text-sm font-medium text-muted-foreground">
                        <Icon className="size-4" />
                        {meta ? t(`assets:home.categoryLabel.${meta.labelKey}`) : subtype}
                      </div>
                      <ul className="divide-y">{grouped.get(subtype)!.map(renderRow)}</ul>
                    </div>
                  );
                })}
              </div>
            )}

            {hasEnvelopeError && (
              <p className="text-sm text-destructive">{errorMessage(save.error)}</p>
            )}

            <div className="flex items-center justify-between">
              <span className="text-sm text-muted-foreground" data-testid="asset-entry-dirty-count">
                {t("bulkEntry.changedCount", { count: dirtyRows.length })}
              </span>
              <div className="flex gap-2">
                <Button type="button" variant="outline" onClick={() => navigate(routes.assets)}>
                  {t("cancel")}
                </Button>
                <Button
                  type="submit"
                  disabled={dirtyRows.length === 0 || save.isPending}
                  data-testid="asset-entry-save"
                >
                  {save.isPending ? t("actions.saving") : t("bulkEntry.save")}
                </Button>
              </div>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
