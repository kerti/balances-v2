import { useState } from "react";
import { PageHeader } from "@/components/common/PageHeader";
import { useNavigate, useSearchParams } from "react-router";
import { useTranslation } from "react-i18next";
import { Wallet } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { MonthPickerPopover } from "@/components/common/MonthPickerPopover";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import {
  thisYearMonth,
  yearMonthsUpToNow,
  monthEndDateCapped,
  monthStartDate,
  pastYearMonthOrNull,
} from "@/lib/dateLimits";
import { formatYearMonth, formatCurrency, formatCurrencyParts } from "@/lib/format";
import { errorMessage } from "@/lib/errorMessage";
import { ownershipLabel } from "@/lib/ownership";
import { useHouseholdMembers } from "@/hooks/useHouseholdMembers";
import { useSession } from "@/hooks/useSession";
import { useIsMobile } from "@/hooks/use-mobile";
import { useEntryList, useBulkSaveSnapshots, type EntryRow } from "@/hooks/useBulkEntry";
import {
  EntryRowDesktop,
  EntryRowDesktopHeader,
  type TickAll,
} from "@/components/entry/EntryRowDesktop";
import { SameCheckbox } from "@/components/entry/entryRow";
import { EntryRowMobile } from "@/components/entry/EntryRowMobile";
import type { EntryGroupConfig } from "@/components/entry/groups";
import {
  minorUnitsShown,
  type EntryField,
  type EntryFieldValues,
  type EntryShape,
} from "@/components/entry/shapes";
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
  // A group can mix shapes (the accrued group: a bond takes two figures, a time
  // deposit one, #844), so every row resolves its own.
  const shapeOf = (row: EntryRow): EntryShape => config.shapeFor?.(row) ?? config.shape;
  // Copy prefix: "bulkEntry" (account wording) unless a group overrides it
  // (investments → holdings wording). Field labels stay under bulkEntry.*.
  const copy = config.copyPrefix ?? "bulkEntry";

  // ?month=YYYY-MM seeds the initial month (the dashboard stale warning links
  // here, #792); only the seed — the picker stays free afterwards.
  const [searchParams] = useSearchParams();
  const [seedMonth] = useState(
    () => pastYearMonthOrNull(searchParams.get("month")) ?? thisYearMonth(),
  );
  const [yearMonth, setYearMonth] = useState(seedMonth);
  // The statement date starts blank and stays the user's to fill (ADR-0046
  // amendment, #790): a date they never stated is not a fact, and nothing
  // computes from it. A blank saves as NULL. When one is picked it must fall
  // within the target month (backend CHECK), which the input's min/max enforce.
  const [asOfDate, setAsOfDate] = useState("");
  // Per-position edits: only the fields the user actually typed. An untouched
  // field falls back to the row's shape prefill (see valueFor). Keyed by
  // position id, then by shape field key ("amount"; or "quantity"/"price…").
  const [edits, setEdits] = useState<Record<string, EntryFieldValues>>({});
  // Carried rows the user marked "Unchanged" (#789): each saves its prefill as
  // this month's Snapshot. Typing into a marked row unmarks it (it is then a
  // plain edit); Undo unmarks it too.
  const [unchangedIds, setUnchangedIds] = useState<ReadonlySet<string>>(new Set());
  const [prevYearMonth, setPrevYearMonth] = useState(yearMonth);

  const list = useEntryList(config, yearMonth);
  // The saved toast's "Next: …" action (ADR-0058 §6, #805) opens the next stale
  // group's entry screen on the month just saved.
  const save = useBulkSaveSnapshots(config, (route, month) => navigate(`${route}?month=${month}`));
  const { data: members } = useHouseholdMembers();
  const { data: me } = useSession();
  // ADR-0050: a single 768px boolean picks which row renderer mounts — one tree
  // in the DOM, one set of testids. All behaviour below stays in this container.
  const isMobile = useIsMobile();

  // Guarded setState-during-render (no useEffect — lint bans setState-in-effect,
  // ADR-0041 follow-up): when the month changes, clear the statement date (a
  // date picked for the old month would fall outside the new one), clear edits,
  // and drop any stale save error.
  if (prevYearMonth !== yearMonth) {
    setPrevYearMonth(yearMonth);
    setAsOfDate("");
    setEdits({});
    setUnchangedIds(new Set());
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
    return shapeOf(row).prefill(row)[fieldKey] ?? "";
  }
  // shownFor is what the input displays: valueFor, except that a carried-forward
  // money value is rounded to the currency's minor units (ADR-0056). It is for
  // display only. isDirty and the save body read valueFor/mergedValues, so a
  // prefill shown rounded never makes a row dirty or changes what is sent.
  function shownFor(row: EntryRow, f: EntryField): string {
    const v = valueFor(row, f.key);
    if (!f.money || edits[row.position_id]?.[f.key] !== undefined) return v;
    return minorUnitsShown(v, row.currency);
  }
  // The merged field values for a row: prefill overlaid with the user's edits.
  function mergedValues(row: EntryRow): EntryFieldValues {
    const out: EntryFieldValues = {};
    for (const f of shapeOf(row).fields) out[f.key] = valueFor(row, f.key).trim();
    return out;
  }
  // A row is dirty when its values are complete (all required fields present)
  // and at least one differs from the carried-forward prefill. An incomplete
  // qty×price pair (only one of quantity/price typed) is not yet saveable.
  function isDirty(row: EntryRow): boolean {
    const values = mergedValues(row);
    if (!shapeOf(row).complete(values)) return false;
    const pf = shapeOf(row).prefill(row);
    return shapeOf(row).fields.some((f) => values[f.key] !== (pf[f.key] ?? "").trim());
  }

  // A row offering the "same" checkbox: carried from an earlier month (none yet
  // this month), complete, and not edited. A marked row stays markable so it can
  // be unticked.
  function isMarkable(row: EntryRow): boolean {
    return (
      Boolean(row.carried_from) &&
      row.carried_from !== yearMonth &&
      shapeOf(row).complete(mergedValues(row)) &&
      !isDirty(row)
    );
  }
  // The tick-all checkbox over a set of rows: hidden with nothing markable,
  // checked when all are marked, indeterminate when only some are.
  function tickAllFor(scope: EntryRow[], testId: string): TickAll {
    const markable = scope.filter(isMarkable);
    const marked = markable.filter((r) => unchangedIds.has(r.position_id)).length;
    return {
      visible: markable.length > 0,
      checked: markable.length > 0 && marked === markable.length,
      indeterminate: marked > 0 && marked < markable.length,
      onChange: (checked) =>
        setMarked(
          markable.map((r) => r.position_id),
          checked,
        ),
      testId,
    };
  }
  // A marked row is saved even though its values equal the prefill. Changed rows
  // are only the ones that differ (the dot, ring, and "N changed").
  function isSaved(row: EntryRow): boolean {
    return (
      isDirty(row) ||
      (unchangedIds.has(row.position_id) && shapeOf(row).complete(mergedValues(row)))
    );
  }
  const changedCount = rows.filter(isDirty).length;
  const markedCount = rows.filter((r) => !isDirty(r) && isSaved(r)).length;

  const mobileTick = tickAllFor(rows, `${tid}-entry-same-all`);

  const dirtyRows = rows.filter(isSaved).map((r) => ({
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
    setUnchangedIds((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
    setEdits((prev) => {
      const next = { ...prev };
      delete next[id];
      return next;
    });
  }

  function setMarked(ids: string[], marked: boolean) {
    setUnchangedIds((prev) => {
      const next = new Set(prev);
      for (const id of ids) {
        if (marked) next.add(id);
        else next.delete(id);
      }
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
    const shape = shapeOf(row);
    const fieldValues: EntryFieldValues = {};
    for (const f of shape.fields) fieldValues[f.key] = shownFor(row, f);

    let derived: { symbol: string; value: string | null } | null = null;
    let warning = false;
    if (shape.derived) {
      const d = shape.derived(mergedValues(row));
      // Pass "" for an incomplete row so formatCurrencyParts returns value:null
      // (the symbol still resolves) — the renderer shows "—" but keeps the column.
      derived = formatCurrencyParts(d.valid ? d.amount : "", row.currency);
      warning = d.valid && Boolean(d.below);
    }

    let when: EntryWhen;
    if (row.carried_from === yearMonth) {
      // A snapshot already exists for the chosen month — the prefill IS this
      // month's value, so editing it overwrites (upsert).
      const pf = shape.prefill(row);
      const current = shape.current
        ? shape.current(pf)
        : shape.derived
          ? shape.derived(pf).amount
          : (pf.amount ?? "");
      when = { kind: "overwrite", current: formatCurrency(current, row.currency) };
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
      unchanged: unchangedIds.has(row.position_id) && !isDirty(row),
      ownership: ownershipLabel(row.ownership_type, row.sole_owner_user_id, members, me),
      when,
      error: Boolean(rowErrors[row.position_id]),
      fieldValues,
      derived,
      warning,
    };
  }

  function renderRow(row: EntryRow) {
    return (
      <RowRenderer
        key={row.position_id}
        view={rowView(row)}
        shape={shapeOf(row)}
        tid={tid}
        copy={copy}
        onUnchanged={(checked) => setMarked([row.position_id], checked)}
        onFieldChange={(fieldKey, value) => {
          setUnchangedIds((prev) => {
            if (!prev.has(row.position_id)) return prev;
            const next = new Set(prev);
            next.delete(row.position_id);
            return next;
          });
          setEdits((prev) => ({
            ...prev,
            [row.position_id]: { ...(prev[row.position_id] ?? {}), [fieldKey]: value },
          }));
        }}
        onReset={() => resetRow(row.position_id)}
      />
    );
  }

  function submit(e: React.FormEvent) {
    e.preventDefault();
    if (dirtyRows.length === 0) return;
    save.mutate(
      { year_month: yearMonth, as_of_date: asOfDate, rows: dirtyRows },
      {
        onSuccess: (result) => {
          if (!result.ok) return;
          navigate(config.backRoute);
          // Back lands at the top, not wherever this long form was scrolled.
          window.scrollTo(0, 0);
        },
      },
    );
  }

  return (
    <div className="mx-auto max-w-4xl md:p-4">
      <PageHeader
        variant="sub-page"
        back={{
          label: t("common:actions.back"),
          onClick: () => navigate(config.backRoute),
          testId: `${tid}-entry-back`,
        }}
        title={t(`${copy}.title`)}
        subtitle={t(`${copy}.description`)}
      />
      <Card className="mt-4">
        <CardContent>
          <form onSubmit={submit} className="space-y-4">
            {/* items-end keeps both inputs on one line even when a label wraps
                to two rows on a narrow phone ("Statement date" vs "Month"). */}
            <div className="grid items-end gap-3 md:grid-cols-2">
              <div className="grid gap-2">
                <Label htmlFor="bulk-year-month">{t("fields.month")}</Label>
                <MonthPickerPopover
                  id="bulk-year-month"
                  months={yearMonthsUpToNow()}
                  selected={yearMonth}
                  onSelect={setYearMonth}
                  fullWidth
                  testId={`${tid}-entry-month`}
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

            {isMobile && mobileTick.visible && (
              <label className="flex min-h-11 items-center justify-end gap-2 text-sm text-muted-foreground">
                <SameCheckbox
                  checked={mobileTick.checked}
                  indeterminate={mobileTick.indeterminate}
                  onChange={mobileTick.onChange}
                  label={t("bulkEntry.sameAll")}
                  testId={mobileTick.testId}
                  className="size-5"
                />
                {t("bulkEntry.allSame")}
              </label>
            )}

            {list.isPending ? (
              <p className="text-sm text-muted-foreground">{t("loading")}</p>
            ) : rows.length === 0 ? (
              <p className="text-sm text-muted-foreground" data-testid={`${tid}-entry-empty`}>
                {t(`${copy}.empty`)}
              </p>
            ) : flat ? (
              <div>
                {!isMobile && (
                  <div className="mb-1 flex justify-end">
                    <EntryRowDesktopHeader
                      shape={shapeOf(rows[0])}
                      tickAll={tickAllFor(rows, `${tid}-entry-same-all`)}
                    />
                  </div>
                )}
                <ul className="divide-y">{rows.map(renderRow)}</ul>
              </div>
            ) : (
              <div className="space-y-4">
                {orderedSubtypes.map((subtype) => {
                  const meta = config.subtypeMeta[subtype];
                  const Icon = meta?.icon ?? Wallet;
                  return (
                    <div key={subtype} data-testid={`${tid}-entry-group-${subtype}`}>
                      {/* The group title shares its line with the desktop column
                          headers (once per investment type; mobile labels each
                          field inline). Its right edge holds the "Same" column. */}
                      <div className="mb-1 flex items-center gap-3 text-sm font-medium text-muted-foreground">
                        <div className="flex min-w-0 flex-1 items-center gap-2">
                          <Icon className="size-4 shrink-0" />
                          <span className="truncate">
                            {meta && config.labelNs
                              ? t(`${config.labelNs}:home.categoryLabel.${meta.labelKey}`)
                              : subtype}
                          </span>
                        </div>
                        {!isMobile && (
                          <EntryRowDesktopHeader
                            shape={shapeOf(grouped.get(subtype)![0])}
                            tickAll={tickAllFor(
                              grouped.get(subtype)!,
                              `${tid}-entry-same-all-${subtype}`,
                            )}
                          />
                        )}
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
              <div className="flex flex-wrap items-center gap-x-2">
                <span
                  className="text-sm text-muted-foreground"
                  data-testid={`${tid}-entry-dirty-count`}
                >
                  {t("bulkEntry.changedCount", { count: changedCount })}
                </span>
                {markedCount > 0 && (
                  <span
                    className="text-sm text-muted-foreground"
                    data-testid={`${tid}-entry-unchanged-count`}
                  >
                    {t("bulkEntry.unchangedCount", { count: markedCount })}
                  </span>
                )}
              </div>
              <div className="flex flex-wrap justify-end gap-2">
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
