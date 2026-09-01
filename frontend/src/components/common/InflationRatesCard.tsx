import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, Pencil, Trash2, X } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { PaginationControls } from "@/components/common/PaginationControls";
import { errorMessage } from "@/lib/errorMessage";
import {
  useInflationRates,
  useCreateInflationRate,
  useUpdateInflationRate,
  useDeleteInflationRate,
} from "@/hooks/useInflationRates";
import { useIsMobile } from "@/hooks/use-mobile";
import { formatPercent, formatYearMonth } from "@/lib/format";
import type { InflationRate } from "@/api/types";

// One year of monthly figures per page — matches the Income screen's page size.
const PAGE_SIZE = 12;

// InflationRatesCard is the Settings ▸ Inflation Rates subpage's sole content:
// the manual monthly table (ADR-0048's "FX-like store"). The assumed-annual
// fallback lives on the Settings home page (Household section) instead — it's
// a single household preference, not a row-based lookup table like this one.
// Rates are annualized (YoY) percentages and may be negative (deflation). No
// card-level title/description — the page header (SettingsInflationRatesScreen)
// owns that.
//
// Mobile–web layout divergence (ADR-0050 B3b, #511): the add form is shared, but
// the entered rows split at the renderer — `useIsMobile` (768px) mounts stacked
// cards on phones and the wide table on desktop, so only one tree is ever in the
// DOM. Both leaves are fed the same rows and share the `inflation-rate-row` testid.
//
// A row's rate is editable in place (the month is the row's identity — the
// PATCH takes only `rate`): Edit swaps the rate into an input with Save/Cancel;
// only one row edits at a time (`editingId`).
export function InflationRatesCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: rates, isPending } = useInflationRates();
  const createRate = useCreateInflationRate();
  const updateRate = useUpdateInflationRate();
  const deleteRate = useDeleteInflationRate();
  const isMobile = useIsMobile();

  const [month, setMonth] = useState("");
  const [rate, setRate] = useState("");

  const [editingId, setEditingId] = useState<string | null>(null);
  const [editRate, setEditRate] = useState("");

  const [page, setPage] = useState(1);
  const totalPages = Math.max(1, Math.ceil((rates?.length ?? 0) / PAGE_SIZE));
  const effectivePage = Math.min(page, totalPages);
  const pageRates = (rates ?? []).slice((effectivePage - 1) * PAGE_SIZE, effectivePage * PAGE_SIZE);

  const add = () => {
    createRate.mutate(
      { year_month: month, rate },
      {
        onSuccess: () => {
          setMonth("");
          setRate("");
        },
      },
    );
  };

  const startEdit = (r: InflationRate) => {
    setEditingId(r.id);
    setEditRate(r.rate);
  };
  const cancelEdit = () => setEditingId(null);
  const saveEdit = (id: string) => {
    updateRate.mutate({ id, rate: editRate }, { onSuccess: () => setEditingId(null) });
  };

  // A rate may be negative (deflation) or zero; only require a parseable number.
  const rateValid = (v: string) => v.trim() !== "" && !Number.isNaN(Number(v));
  const canAdd = month !== "" && rateValid(rate);
  const canSave = rateValid(editRate);

  return (
    <Card data-testid="inflation-rates-card">
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="inflation-month">{t("inflation.month")}</Label>
            <Input
              id="inflation-month"
              type="month"
              className="w-40"
              value={month}
              onChange={(e) => setMonth(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="inflation-rate">{t("inflation.rate")}</Label>
            <Input
              id="inflation-rate"
              inputMode="decimal"
              className="w-36"
              // Example numeric rate; not translatable copy.
              placeholder={"3.5"}
              value={rate}
              onChange={(e) => setRate(e.target.value)}
            />
          </div>
          <Button onClick={add} disabled={!canAdd || createRate.isPending}>
            {t("inflation.addRate")}
          </Button>
        </div>

        {createRate.isError && (
          <p className="text-sm text-destructive">{errorMessage(createRate.error)}</p>
        )}

        {updateRate.isError && (
          <p className="text-sm text-destructive">{errorMessage(updateRate.error)}</p>
        )}

        {isPending && <p className="text-sm text-muted-foreground">{t("common:loading")}</p>}

        {rates && rates.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("inflation.empty")}</p>
        )}

        {rates &&
          rates.length > 0 &&
          (isMobile ? (
            <div className="space-y-2" data-testid="inflation-rate-cards">
              {pageRates.map((r) => (
                <InflationRateCard
                  key={r.id}
                  rate={r}
                  editLabel={t("common:actions.edit")}
                  deleteLabel={t("common:delete")}
                  saveLabel={t("common:save")}
                  cancelLabel={t("common:cancel")}
                  isEditing={editingId === r.id}
                  editValue={editRate}
                  onEditChange={setEditRate}
                  onEditStart={() => startEdit(r)}
                  onSave={() => saveEdit(r.id)}
                  onCancel={cancelEdit}
                  canSave={canSave}
                  isSaving={updateRate.isPending}
                  onDelete={() => deleteRate.mutate(r.id)}
                />
              ))}
            </div>
          ) : (
            <Table data-testid="inflation-rate-table">
              <TableHeader>
                <TableRow>
                  <TableHead>{t("inflation.month")}</TableHead>
                  <TableHead className="text-right">{t("inflation.rate")}</TableHead>
                  <TableHead className="w-24"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pageRates.map((r) => {
                  const editing = editingId === r.id;
                  return (
                    <TableRow key={r.id} data-testid="inflation-rate-row">
                      <TableCell>{formatYearMonth(r.year_month)}</TableCell>
                      <TableCell
                        className="text-right tabular-nums"
                        data-testid="inflation-rate-value"
                      >
                        {editing ? (
                          <Input
                            inputMode="decimal"
                            className="w-28 ml-auto text-right"
                            aria-label={t("inflation.rate")}
                            value={editRate}
                            onChange={(e) => setEditRate(e.target.value)}
                          />
                        ) : (
                          formatPercent(r.rate)
                        )}
                      </TableCell>
                      <TableCell>
                        {editing ? (
                          <div className="flex gap-1">
                            <Button
                              variant="ghost"
                              size="icon"
                              aria-label={t("common:save")}
                              onClick={() => saveEdit(r.id)}
                              disabled={!canSave || updateRate.isPending}
                            >
                              <Check className="size-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              aria-label={t("common:cancel")}
                              onClick={cancelEdit}
                            >
                              <X className="size-4" />
                            </Button>
                          </div>
                        ) : (
                          <div className="flex gap-1">
                            <Button
                              variant="ghost"
                              size="icon"
                              aria-label={t("common:actions.edit")}
                              onClick={() => startEdit(r)}
                            >
                              <Pencil className="size-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              aria-label={t("common:delete")}
                              onClick={() => deleteRate.mutate(r.id)}
                            >
                              <Trash2 className="size-4" />
                            </Button>
                          </div>
                        )}
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          ))}

        {totalPages > 1 && (
          <PaginationControls page={effectivePage} totalPages={totalPages} onPageChange={setPage} />
        )}
      </CardContent>
    </Card>
  );
}

// Mobile leaf (ADR-0050 "wide table → stacked cards"): one card per inflation
// row. The rate — the annualized figure the user came to check — is promoted to
// the headline (a trailing "%" makes the unit explicit at a glance) with the
// month below. Edit swaps the headline for an input with Save/Cancel; the
// edit/delete controls are icon buttons sized to the 44px tap floor
// (INV-PRESENTATION-08).
function InflationRateCard({
  rate,
  editLabel,
  deleteLabel,
  saveLabel,
  cancelLabel,
  isEditing,
  editValue,
  onEditChange,
  onEditStart,
  onSave,
  onCancel,
  canSave,
  isSaving,
  onDelete,
}: {
  rate: InflationRate;
  editLabel: string;
  deleteLabel: string;
  saveLabel: string;
  cancelLabel: string;
  isEditing: boolean;
  editValue: string;
  onEditChange: (v: string) => void;
  onEditStart: () => void;
  onSave: () => void;
  onCancel: () => void;
  canSave: boolean;
  isSaving: boolean;
  onDelete: () => void;
}) {
  return (
    <div className="flex items-center gap-3 rounded-lg border p-3" data-testid="inflation-rate-row">
      <div className="min-w-0 flex-1">
        {isEditing ? (
          <Input
            inputMode="decimal"
            aria-label={editLabel}
            value={editValue}
            onChange={(e) => onEditChange(e.target.value)}
          />
        ) : (
          <div className="text-lg font-semibold tabular-nums" data-testid="inflation-rate-value">
            {formatPercent(rate.rate)}
          </div>
        )}
        <div className="text-sm text-muted-foreground">{formatYearMonth(rate.year_month)}</div>
      </div>
      {isEditing ? (
        <div className="flex shrink-0 gap-1">
          <Button size="sm" onClick={onSave} disabled={!canSave || isSaving}>
            {saveLabel}
          </Button>
          <Button variant="ghost" size="sm" onClick={onCancel}>
            {cancelLabel}
          </Button>
        </div>
      ) : (
        <div className="flex shrink-0 gap-1">
          <Button
            variant="ghost"
            size="icon"
            className="size-11"
            aria-label={editLabel}
            onClick={onEditStart}
          >
            <Pencil className="size-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="size-11"
            aria-label={deleteLabel}
            onClick={onDelete}
          >
            <Trash2 className="size-4" />
          </Button>
        </div>
      )}
    </div>
  );
}
