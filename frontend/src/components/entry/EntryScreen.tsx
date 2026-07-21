import { useState } from "react";
import { useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import { Wallet } from "lucide-react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { thisYearMonth, monthEndDateCapped, monthStartDate } from "@/lib/dateLimits";
import { formatYearMonth, formatCurrency } from "@/lib/format";
import { errorMessage } from "@/lib/errorMessage";
import { ownershipLabel } from "@/lib/ownership";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import { useIsMobile } from "@/hooks/use-mobile";
import { useEntryList, useBulkSaveSnapshots, type EntryRow } from "@/hooks/useBulkEntry";
import { EntryRowDesktop, EntryRowDesktopHeader } from "@/components/entry/EntryRowDesktop";
import { EntryRowMobile } from "@/components/entry/EntryRowMobile";
import type { EntryGroupConfig } from "@/components/entry/groups";
import type { EntryFieldValues } from "@/components/entry/shapes";
import type { EntryRowView, EntryWhen } from "@/components/entry/entryRow";

// EntryScreen is the bulk monthly-entry view for one amount-only group
// (ADR-0046): one screen listing every position eligible for a chosen month,
// each with its last value carried forward, a batch-level "when" control, and
// one Save. Only rows the user actually changed are sent (dirty-only);
// untouched positions ride the carry-forward rule. The group's endpoints,
// grouping, labels, and testid prefix arrive as `config`, so Asset (#421),
// Liability, and Receivable (#422) share this one component — the read-only
// ADR-0043 list core stays untouched; this is launched *from* the list.
export function EntryScreen({ config }: { config: EntryGroupConfig }) {
  const { t } = useTranslation(["common", "assets", "liabilities", "investments"]);
  const navigate = useNavigate();
  const tid = config.testidPrefix;
  const shape = config.shape;
  // Copy prefix: "bulkEntry" (account wording) unless a group overrides it
  // (investments → holdings wording). Field labels stay under bulkEntry.*.
  const copy = config.copyPrefix ?? "bulkEntry";

  const [yearMonth, setYearMonth] = useState(thisYearMonth());
  // as_of_date must fall within the target month (backend CHECK), so it seeds
  // to the end of that month (capped at today) rather than the single-snapshot
  // carryover_date_mode date, which does not generalise to an arbitrarily
  // chosen target month.
  const [asOfDate, setAsOfDate] = useState(monthEndDateCapped(thisYearMonth()));
  // Per-position edits: only the fields the user actually typed. An untouched
  // field falls back to the row's shape prefill (see valueFor). Keyed by
  // position id, then by shape field key ("amount"; or "quantity"/"price…").
  const [edits, setEdits] = useState<Record<string, EntryFieldValues>>({});
  const [prevYearMonth, setPrevYearMonth] = useState(yearMonth);

  const list = useEntryList(config, yearMonth);
  const save = useBulkSaveSnapshots(config);
  const { data: members } = useHouseholdMembers();
  const { data: me } = useSession();
  // ADR-0050: a single 768px boolean picks which row renderer mounts — one tree
  // in the DOM, one set of testids. All behaviour below stays in this container.
  const isMobile = useIsMobile();

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

  // Per-row rejections from a 422 (ADR-0046) come back as data, keyed by
  // position.
  const rowErrors: Record<string, string> = {};
  if (save.data && !save.data.ok) {
    for (const e of save.data.errors) rowErrors[e.position_id] = e.code;
  }
  // A thrown error is any non-2xx that isn't the per-row 422.
  const hasEnvelopeError = save.isError;

  // valueFor returns the shown value of one field: the user's edit if any, else
  // the row's shape prefill for that field ("" when the position has no
  // history).
  function valueFor(row: EntryRow, fieldKey: string): string {
    const edited = edits[row.position_id]?.[fieldKey];
    if (edited !== undefined) return edited;
    return shape.prefill(row)[fieldKey] ?? "";
  }
  // The merged field values for a row: prefill overlaid with the user's edits.
  function mergedValues(row: EntryRow): EntryFieldValues {
    const out: EntryFieldValues = {};
    for (const f of shape.fields) out[f.key] = valueFor(row, f.key).trim();
    return out;
  }
  // A row is dirty when its values are complete (all required fields present)
  // and at least one differs from the carried-forward prefill. An incomplete
  // qty×price pair (only one of quantity/price typed) is not yet saveable.
  function isDirty(row: EntryRow): boolean {
    const values = mergedValues(row);
    if (!shape.complete(values)) return false;
    const pf = shape.prefill(row);
    return shape.fields.some((f) => values[f.key] !== (pf[f.key] ?? "").trim());
  }

  const dirtyRows = rows.filter(isDirty).map((r) => ({
    position_id: r.position_id,
    currency: r.currency,
    fields: mergedValues(r),
  }));

  // Group eligible rows by subtype (rows arrive pre-ordered by subtype then
  // name). A flat group (empty subtypeOrder) renders one ungrouped section.
  const grouped = new Map<string, EntryRow[]>();
  for (const r of rows) {
    const g = grouped.get(r.subtype) ?? [];
    g.push(r);
    grouped.set(r.subtype, g);
  }
  const orderedSubtypes = [
    ...config.subtypeOrder.filter((s) => grouped.has(s)),
    ...[...grouped.keys()].filter((s) => !config.subtypeOrder.includes(s)),
  ];
  const flat = config.subtypeOrder.length === 0;

  // resetRow drops the user's override so the field falls back to its
  // carried-forward prefill (empty for a position with no history).
  function resetRow(id: string) {
    setEdits((prev) => {
      const next = { ...prev };
      delete next[id];
      return next;
    });
  }

  // The renderer differs only in leaf layout (ADR-0050); both take the same
  // presentation-neutral EntryRowView and the same field-change/reset callbacks.
  const RowRenderer = isMobile ? EntryRowMobile : EntryRowDesktop;

  // rowView projects one EntryRow into the presentation-neutral shape both
  // renderers consume — resolving the container-only knowledge (edits, members,
  // the chosen month, the shape's derived value) into plain display data.
  function rowView(row: EntryRow): EntryRowView {
    const fieldValues: EntryFieldValues = {};
    for (const f of shape.fields) fieldValues[f.key] = valueFor(row, f.key);

    let derived: string | null = null;
    if (shape.derived) {
      const d = shape.derived(mergedValues(row));
      derived = d.valid ? formatCurrency(d.amount, row.currency) : "—";
    }

    let when: EntryWhen;
    if (row.carried_from === yearMonth) {
      // A snapshot already exists for the chosen month — the prefill IS this
      // month's value, so editing it overwrites (upsert).
      when = { kind: "overwrite" };
    } else if (row.carried_from) {
      when = { kind: "carried", month: formatYearMonth(`${row.carried_from}-01T00:00:00Z`) };
    } else {
      when = { kind: "none" };
    }

    return {
      positionId: row.position_id,
      displayName: row.display_name,
      currency: row.currency,
      dirty: isDirty(row),
      ownership: ownershipLabel(row.ownership_type, row.sole_owner_user_id, members, me),
      when,
      error: Boolean(rowErrors[row.position_id]),
      fieldValues,
      derived,
    };
  }

  function renderRow(row: EntryRow) {
    return (
      <RowRenderer
        key={row.position_id}
        view={rowView(row)}
        shape={shape}
        tid={tid}
        copy={copy}
        onFieldChange={(fieldKey, value) =>
          setEdits((prev) => ({
            ...prev,
            [row.position_id]: { ...(prev[row.position_id] ?? {}), [fieldKey]: value },
          }))
        }
        onReset={() => resetRow(row.position_id)}
      />
    );
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (dirtyRows.length === 0) return;
    save.mutate(
      { year_month: yearMonth, as_of_date: asOfDate, rows: dirtyRows },
      { onSuccess: (result) => result.ok && navigate(config.backRoute) },
    );
  }

  return (
    <div className="mx-auto max-w-3xl p-4">
      <Button
        variant="ghost"
        size="sm"
        onClick={() => navigate(config.backRoute)}
        className="-ml-2 mb-1"
        data-testid={`${tid}-entry-back`}
      >
        {t("common:actions.back")}
      </Button>
      <Card>
        <CardHeader>
          <CardTitle>{t(`${copy}.title`)}</CardTitle>
          <CardDescription>{t(`${copy}.description`)}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={submit} className="space-y-4">
            {/* items-end keeps both inputs on one line even when a label wraps
                to two rows on a narrow phone ("Statement date" vs "Month"). */}
            <div className="grid grid-cols-2 items-end gap-3">
              <div className="grid gap-2">
                <Label htmlFor="bulk-year-month">{t("fields.month")}</Label>
                <Input
                  id="bulk-year-month"
                  type="month"
                  max={thisYearMonth()}
                  value={yearMonth}
                  onChange={(e) => setYearMonth(e.target.value)}
                  data-testid={`${tid}-entry-month`}
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
                  data-testid={`${tid}-entry-asof`}
                />
              </div>
            </div>

            {list.isPending ? (
              <p className="text-sm text-muted-foreground">{t("loading")}</p>
            ) : rows.length === 0 ? (
              <p className="text-sm text-muted-foreground" data-testid={`${tid}-entry-empty`}>
                {t(`${copy}.empty`)}
              </p>
            ) : flat ? (
              <div>
                {!isMobile && <EntryRowDesktopHeader shape={shape} />}
                <ul className="divide-y">{rows.map(renderRow)}</ul>
              </div>
            ) : (
              <div className="space-y-4">
                {orderedSubtypes.map((subtype) => {
                  const meta = config.subtypeMeta[subtype];
                  const Icon = meta?.icon ?? Wallet;
                  return (
                    <div key={subtype} data-testid={`${tid}-entry-group-${subtype}`}>
                      <div className="mb-1 flex items-center gap-2 text-sm font-medium text-muted-foreground">
                        <Icon className="size-4" />
                        {meta && config.labelNs
                          ? t(`${config.labelNs}:home.categoryLabel.${meta.labelKey}`)
                          : subtype}
                      </div>
                      {/* Column headers once per investment type (desktop only;
                          mobile labels each field inline). Null for amount-only. */}
                      {!isMobile && <EntryRowDesktopHeader shape={shape} />}
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
              <span
                className="text-sm text-muted-foreground"
                data-testid={`${tid}-entry-dirty-count`}
              >
                {t("bulkEntry.changedCount", { count: dirtyRows.length })}
              </span>
              <div className="flex gap-2">
                <Button type="button" variant="outline" onClick={() => navigate(config.backRoute)}>
                  {t("cancel")}
                </Button>
                <Button
                  type="submit"
                  disabled={dirtyRows.length === 0 || save.isPending}
                  data-testid={`${tid}-entry-save`}
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
