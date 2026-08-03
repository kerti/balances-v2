import { useState } from "react";
import { Archive } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { errorMessage } from "@/lib/errorMessage";
import { formatCurrency } from "@/lib/format";
import { computeCostBasis } from "@/lib/costBasis";
import { useUpdateLifecycle, type LifecycleSettlement } from "@/hooks/useLifecycle";
import { useInvestmentTransactions } from "@/hooks/useInvestmentTransactions";
import { useInvestmentSnapshots } from "@/hooks/useInvestmentSnapshots";
import {
  settlementKind,
  statusOptions,
  type InvestmentSubtype,
  type LifecycleGroup,
} from "@/lib/lifecycle";

// todayISO returns YYYY-MM-DD in the local timezone. toISOString() would shift
// users east of UTC into yesterday for the first hours of their day.
function todayISO(): string {
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

// The settlement fields, as free text. Kept as strings (not numbers) so a
// half-typed value round-trips through the input unchanged, matching every other
// money form on the app.
type SettlementForm = {
  quantity: string;
  price_per_unit: string;
  principal_amount: string;
  interest_amount: string;
};

const EMPTY_SETTLEMENT: SettlementForm = {
  quantity: "",
  price_per_unit: "",
  principal_amount: "",
  interest_amount: "",
};

function num(value: string): number {
  const n = Number(value);
  return Number.isFinite(n) ? n : 0;
}

type Props = {
  group: LifecycleGroup;
  id: string;
  listKey: string;
  currentStatus: string;
  currentTerminatedAt: string | null;
  currentNote: string | null;
  // Investment-only (ADR-0052 §6). Present ⇒ this position's terminal statuses
  // are narrowed to the ones its transaction matrix can settle, and terminating
  // it captures the settling Sell/Maturity. Absent for Asset/Liability/Receivable.
  investmentSubtype?: InvestmentSubtype;
  currency?: string;
};

// Dedicated "change lifecycle status" dialog (ADR-0009): a separate action from
// Edit, operating on the parent table via PATCH /{group}/{id}/lifecycle. The
// backend enforces the biconditional (status=active ⟺ terminated_at IS NULL);
// we mirror it here so the date field appears only when terminating and
// auto-fills today the moment a terminal status is picked.
//
// For Investments it also captures the settlement (ADR-0052 §6) — the Sell or
// Maturity that says where the position's value went — which the backend writes
// in the same database transaction as the flip. Without it a terminated position
// books its whole value as a negative Investment Return, which is the truth only
// when the money really was lost (issue #587).
export function TerminatePositionDialog({
  group,
  id,
  listKey,
  currentStatus,
  currentTerminatedAt,
  currentNote,
  investmentSubtype,
  currency,
}: Props) {
  const { t } = useTranslation("common");
  const [open, setOpen] = useState(false);
  const mutation = useUpdateLifecycle(group, id, listKey);

  // Both are Investment-only reads, disabled by a null id for the other three
  // groups. The ledger sizes the default sale and reveals a sale the user
  // already recorded; the snapshots supply the last marked price / accrual.
  const settles = investmentSubtype !== undefined;
  const { data: transactions } = useInvestmentTransactions(settles ? id : null);
  const { data: snapshots } = useInvestmentSnapshots(settles ? id : null);

  const [status, setStatus] = useState(currentStatus);
  const [terminatedAt, setTerminatedAt] = useState(
    currentTerminatedAt ? currentTerminatedAt.slice(0, 10) : "",
  );
  const [note, setNote] = useState(currentNote ?? "");
  const [settlement, setSettlement] = useState<SettlementForm>(EMPTY_SETTLEMENT);
  const [writeOff, setWriteOff] = useState(false);

  const wasActive = currentStatus === "active";
  const isActive = status === "active";

  // The settlement is captured only on the active → terminal edge: re-asserting
  // a terminal status (correcting a date or note) must not book a second sale,
  // and the backend refuses one anyway.
  const kind =
    investmentSubtype && wasActive && !isActive ? settlementKind(investmentSubtype, status) : null;

  // A sale the user already recorded by hand in the termination month. Several
  // partial Sells in one month are legitimate, so this never blocks — it just
  // stops the dialog offering to book a duplicate of the one that is already
  // there.
  const termMonth = terminatedAt.slice(0, 7);
  const alreadySettled =
    kind !== null &&
    (transactions ?? []).some(
      (txn) =>
        (txn.transaction_type === "sell" || txn.transaction_type === "maturity") &&
        txn.transaction_date.slice(0, 7) === termMonth,
    );
  const capturing = kind !== null && !alreadySettled;

  // Defaults, recomputed on open rather than held in state so they follow a
  // status change (a Bond can settle either way).
  function defaultsFor(next: "sell" | "maturity" | null): SettlementForm {
    if (next === null) return EMPTY_SETTLEMENT;
    const latest = [...(snapshots ?? [])].sort((a, b) =>
      b.year_month.localeCompare(a.year_month),
    )[0];
    if (next === "sell") {
      // Held quantity comes from the ledger, not the snapshot: the ledger is
      // what the cost basis replays, so sizing the closing Sell from it is what
      // drives the basis to zero.
      const heldQty = computeCostBasis(transactions ?? []).heldQty;
      return {
        ...EMPTY_SETTLEMENT,
        quantity: String(heldQty),
        // A position holding nothing has no price to state, and 0 × 0 = 0 is the
        // truthful settlement — so it defaults to 0 and the form stays
        // submittable. A position that DOES hold something but has never been
        // marked is left blank and required instead: what it sold for is exactly
        // the judgement this capture exists to take, and quietly defaulting it to
        // 0 would book a real sale as a total loss.
        price_per_unit: latest?.price_per_unit ?? (heldQty === 0 ? "0" : ""),
      };
    }
    const total = num(latest?.amount ?? "0");
    const accrued = num(latest?.accrued_interest ?? "0");
    const interest = Math.max(0, Math.min(accrued, total));
    return {
      ...EMPTY_SETTLEMENT,
      principal_amount: String(total - interest),
      interest_amount: String(interest),
    };
  }

  function reset() {
    setStatus(currentStatus);
    setTerminatedAt(currentTerminatedAt ? currentTerminatedAt.slice(0, 10) : "");
    setNote(currentNote ?? "");
    setSettlement(EMPTY_SETTLEMENT);
    setWriteOff(false);
    mutation.reset();
  }

  function close() {
    setOpen(false);
    reset();
  }

  function onStatusChange(next: string) {
    setStatus(next);
    // Picking a terminal status auto-fills today (require + default today).
    // Going back to active clears the date so the biconditional holds.
    if (next === "active") {
      setTerminatedAt("");
    } else if (!terminatedAt) {
      setTerminatedAt(todayISO());
    }
    setWriteOff(false);
    setSettlement(
      defaultsFor(
        investmentSubtype && wasActive && next !== "active"
          ? settlementKind(investmentSubtype, next)
          : null,
      ),
    );
  }

  // The write-off escape (ADR-0052 §5): a position that genuinely lost its value
  // is still settled by a Transaction, at zero. The quantity still leaves the
  // position — only the price is zero — so the cost basis closes out the same
  // way a real sale would.
  function onWriteOffChange(checked: boolean) {
    setWriteOff(checked);
    if (checked) {
      setSettlement((s) => ({
        ...s,
        price_per_unit: "0",
        principal_amount: "0",
        interest_amount: "0",
      }));
    } else {
      setSettlement(defaultsFor(kind));
    }
  }

  const proceeds =
    kind === "sell"
      ? num(settlement.quantity) * num(settlement.price_per_unit)
      : num(settlement.principal_amount) + num(settlement.interest_amount);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    let payload: LifecycleSettlement | undefined;
    if (capturing) {
      payload =
        kind === "sell"
          ? {
              quantity: settlement.quantity,
              price_per_unit: settlement.price_per_unit,
              principal_amount: null,
              interest_amount: null,
            }
          : {
              quantity: null,
              price_per_unit: null,
              principal_amount: settlement.principal_amount,
              interest_amount: settlement.interest_amount,
            };
    }
    mutation.mutate(
      {
        status,
        terminated_at: isActive ? null : terminatedAt,
        termination_note: note.trim() ? note.trim() : null,
        ...(payload ? { settlement: payload } : {}),
      },
      { onSuccess: close },
    );
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? setOpen(true) : close())}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm" data-testid="terminate-position-trigger">
          <Archive className="mr-1 size-4" />
          {wasActive ? t("terminate.closeTrigger") : t("terminate.editTrigger")}
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {wasActive ? t("terminate.closeTitle") : t("terminate.editTitle")}
          </DialogTitle>
          <DialogDescription>{t("terminate.description")}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit} className="space-y-3">
          <div className="grid gap-2">
            <Label htmlFor="lifecycle_status">{t("terminate.statusLabel")}</Label>
            <Select
              id="lifecycle_status"
              value={status}
              onChange={(e) => onStatusChange(e.target.value)}
            >
              {statusOptions(group, investmentSubtype, currentStatus).map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </Select>
          </div>

          {!isActive && (
            <div className="grid gap-2">
              <Label htmlFor="lifecycle_terminated_at">{t("terminate.terminatedOnLabel")}</Label>
              <Input
                id="lifecycle_terminated_at"
                type="date"
                required
                max="9999-12-31"
                value={terminatedAt}
                onChange={(e) => setTerminatedAt(e.target.value)}
              />
            </div>
          )}

          {capturing && (
            <div className="grid gap-3 rounded-md border p-3" data-testid="terminate-settlement">
              <p className="text-sm font-medium">{t("terminate.settlementTitle")}</p>

              {kind === "sell" ? (
                <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
                  <div className="grid gap-2">
                    <Label htmlFor="settlement_quantity">
                      {t("terminate.settlementQuantityLabel")}
                    </Label>
                    <Input
                      id="settlement_quantity"
                      required
                      inputMode="decimal"
                      data-testid="settlement-quantity"
                      value={settlement.quantity}
                      onChange={(e) => setSettlement({ ...settlement, quantity: e.target.value })}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="settlement_price">
                      {t("terminate.settlementPriceLabel", { currency })}
                    </Label>
                    <Input
                      id="settlement_price"
                      required
                      inputMode="decimal"
                      disabled={writeOff}
                      data-testid="settlement-price"
                      value={settlement.price_per_unit}
                      onChange={(e) =>
                        setSettlement({ ...settlement, price_per_unit: e.target.value })
                      }
                    />
                  </div>
                </div>
              ) : (
                <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
                  <div className="grid gap-2">
                    <Label htmlFor="settlement_principal">
                      {t("terminate.settlementPrincipalLabel", { currency })}
                    </Label>
                    <Input
                      id="settlement_principal"
                      required
                      inputMode="decimal"
                      disabled={writeOff}
                      data-testid="settlement-principal"
                      value={settlement.principal_amount}
                      onChange={(e) =>
                        setSettlement({ ...settlement, principal_amount: e.target.value })
                      }
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="settlement_interest">
                      {t("terminate.settlementInterestLabel", { currency })}
                    </Label>
                    <Input
                      id="settlement_interest"
                      required
                      inputMode="decimal"
                      disabled={writeOff}
                      data-testid="settlement-interest"
                      value={settlement.interest_amount}
                      onChange={(e) =>
                        setSettlement({ ...settlement, interest_amount: e.target.value })
                      }
                    />
                  </div>
                </div>
              )}

              <div className="rounded-md bg-muted px-3 py-2 text-sm">
                <span className="text-muted-foreground">{t("terminate.settlementCashIn")}</span>{" "}
                <span className="font-medium" data-testid="settlement-total">
                  {currency ? formatCurrency(String(proceeds), currency) : proceeds}
                </span>
              </div>

              <label className="flex items-center gap-2 text-sm text-muted-foreground">
                <input
                  type="checkbox"
                  className="h-4 w-4"
                  checked={writeOff}
                  data-testid="settlement-write-off"
                  onChange={(e) => onWriteOffChange(e.target.checked)}
                />
                {t("terminate.settlementWriteOff")}
              </label>
            </div>
          )}

          {kind !== null && alreadySettled && (
            <p className="text-sm text-muted-foreground" data-testid="terminate-already-settled">
              {t("terminate.settlementAlreadyRecorded")}
            </p>
          )}

          <div className="grid gap-2">
            <Label htmlFor="lifecycle_note">{t("terminate.noteLabel")}</Label>
            <Input
              id="lifecycle_note"
              value={note}
              onChange={(e) => setNote(e.target.value)}
              placeholder={t("terminate.notePlaceholder")}
            />
          </div>

          {mutation.error && (
            <p className="text-sm text-destructive">{errorMessage(mutation.error)}</p>
          )}

          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={mutation.isPending}>
              {mutation.isPending ? t("actions.saving") : t("save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
