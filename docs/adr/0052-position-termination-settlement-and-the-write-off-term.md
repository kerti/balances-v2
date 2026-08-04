# Position termination settlement and the write-off term

Terminating a Position removes its value from net worth, but nothing guaranteed the offsetting flow
was ever recorded. The comprehensive income identity ([[adr-0008]]) had no term that could absorb a
Position's value simply vanishing, so the whole amount landed in the derived Living Expenses residual
— sometimes driving a month **negative**, which reads to a household as "our net worth went up and
nothing explains it" (issue #576).

This ADR settles what a terminal status *means* across all four Position groups. Two independent
defects are fixed: a **timing** defect (the two legs of a cash-settled termination land in different
months) and a **missing term** (non-cash terminations have no income-statement counterpart at all).
It extends [[adr-0009]]'s lifecycle rules and [[adr-0008]]'s identity; neither is superseded.

## The two defects

**Timing.** `terminatedBefore` is `idx > monthIndex(terminated_at)` — a terminated Position
contributes through its termination month at its last carried value, then drops out from month+1
(INV-FINANCE-05, by design). Investment already avoided the consequence because [[adr-0009]]/#25
writes a truthful 0-value close snapshot on a terminal flip. Asset, Liability and Receivable write no
close snapshot at all, so the cash leg lands in month M (the bank moves when the user records it)
while the Position leg drops in M+1. The residual is wrong in **both** months, by equal and opposite
amounts:

```
Liability, last snapshot 20 at month M, paid_off at end of M
  M    : bank −20, liability still 20   ⇒ ΔNW = −20   ⇒ residual overstated by 20
  M+1  : liability suppressed            ⇒ ΔNW = +20   ⇒ residual understated by 20
```

**Missing term.** Some terminal statuses have no cash leg at all. A forgiven Liability is a genuine
one-sided net-worth increase; a disposed Asset and a written-off Receivable are genuine one-sided
decreases. No amount of snapshot truthfulness fixes these — the identity is simply short a term, and
the residual silently absorbs them. This directly falsifies the claim CONTEXT.md makes about the
residual being "a genuine cash-spending proxy, not a catch-all."

## The decision

### 1. Every terminal flip writes a truthful 0-value close snapshot, in all four groups

INV-LIFECYCLE-03 is generalised from investment-only to group-agnostic. The rule #25 established for
Investment — **terminate ⇒ 0-value close snapshot; proceeds are transactions** — was never
Investment-specific in its logic, only in its implementation. A Position that left the portfolio
during month M holds nothing at month-end, whatever group it belongs to.

This alone fixes the timing defect for every **cash-settled** terminal status (`closed`, `sold`,
`paid_off`, `collected`, `matured`): both legs now land in month M and ΔNW nets to zero, leaving the
residual untouched.

### 2. The close snapshot displaces by soft-delete + insert, not in-place overwrite

#25's `upsertCloseSnapshot` overwrites the user's termination-month snapshot **in place**, destroying
it unrecoverably; un-terminate then soft-deletes the 0 row, leaving the month with no snapshot rather
than restoring what was there. All four snapshot tables carry a **partial** unique index
`(position_id, year_month) WHERE deleted_at IS NULL`, so the archived row and the 0 row can coexist.

The rule becomes: **soft-delete any live snapshot at the termination month, then insert the 0-value
close snapshot.** Un-terminate reverses it — soft-delete the 0 row, then restore the archived
original **only if no live snapshot exists at that month** (the user may have re-added one while the
Position was terminated; snapshots, unlike transactions, are not blocked on a terminated Position).
Investment is retrofitted to match. INV-LIFECYCLE-04 gets stronger wording as a result: the
reactivated Position carries its *own recorded* value back, not merely "not 0".

This is what makes bulk-correcting historical data a reversible operation rather than a destructive
one.

**Amendment (#602): the pairing is declared, not inferred.** As first shipped, un-terminate found the
archived original by *coincidence* — the archive `UPDATE` and the close `INSERT` share one
transaction, and `now()` is transaction-scoped, so the archived row's `deleted_at` equalled the close
row's `created_at` — plus an `amount <> 0` filter to skip a close row left archived by an earlier
terminate/un-terminate cycle. That worked, but it meant nothing *in the row* distinguished a snapshot
the termination displaced from one the user threw away. Anything entitled to discard the Recycle Bin
would take the fallback with it: a **compacted** backup did exactly that ([[adr-0036]]), keeping the
live close row while dropping the row it pointed at, so a household restored from such a file read a
carried-forward value from an earlier month on its next undo — silently.

The close row now names what it displaced in a `supersedes` column. The link runs close → displaced
rather than the reverse because the partial unique index forces the archive to happen first, so only
the second write can carry the other's id; it also collapses the lookup, since the close row is
already read to tell it apart from a value the user re-recorded while the Position was terminated.
The `amount <> 0` heuristic goes away with it — excluding an earlier cycle's close row is now
structural.

A displaced snapshot is therefore **not user-deleted data**, and compaction carries it. The rejected
alternative was to stop using `deleted_at` for displacement altogether (mark the row `superseded_by`
and widen the unique indexes to match), which would need no backup exception at all — but it puts a
second predicate on ~100 existing `deleted_at IS NULL` reads across the snapshot queries and the
report engine, where a single miss leaks a displaced row into the statements as a silent wrong number.

### 3. A termination-month value change is never a mark change

Asset Value Change is the signed sum of `ΔSnapshot` over `property` and `vehicle` Positions, and its
loop does **not** skip the termination month. Writing a 0 close snapshot would therefore let it
silently absorb the drop — correct by accident for a `disposed` vehicle, and wrong for a `sold` one:

```
property worth 20, sold at M, proceeds land in the bank
  avc = 0 − 20 = −20            ← reads as depreciation
  ΔNW = −20 + 20 = 0
  residual = income + return + (−20) − 0     ← understated by 20
```

So the Asset Value Change loop **excludes the termination month for every terminated Position**.
Scrapping a vehicle is not depreciation. This keeps Asset Value Change meaning exactly what CONTEXT.md
says it means — the non-cash *mark* change of a Position the household still holds — and routes every
termination-month movement to either a cash leg or the write-off term below.

### 4. A new signed term: Write-Off

**Write-Off**: the value a Position carried into its termination month when no cash settled it. The
identity becomes:

```
ΔNet Worth = Earned Income + Investment Return + Asset Value Change + Write-Offs − Living Expenses
```

or as the engine computes the residual:

```
living_expenses = earned_income + investment_return + asset_value_change + write_offs − ΔNW
```

It fires off the **status**, not off the absence of a flow — [[adr-0009]] already split `sold` from
`disposed` precisely to encode "did cash come back", and three of the four groups have no transaction
concept in which an absent flow could be detected:

| Group | Non-cash terminal statuses |
|---|---|
| Asset | `disposed` |
| Liability | `forgiven`, `written_off` |
| Receivable | `written_off` |
| Investment | — (see 5) |

The sign follows the effect on net worth, so a forgiven Liability contributes **positively**. It is
computed as `now − prev` through the same `fx.carried` calls the net-worth pass uses, so it cancels
ΔNW structurally rather than coincidentally, and an unconvertible-currency Position is skipped
consistently in both.

**One signed term, not a gains/losses pair.** Write-Offs is a *correction* term whose job is to keep
the residual honest, not a category a household budgets against. A month containing both a forgiven
debt and a written-off receivable nets toward zero on the line — that is a presentation problem, and
it is solved by rendering the constituent Positions beneath it, not by a second column.

### 5. Investment needs no write-off status

A written-off investment is already expressible, and already correct: it is a genuine negative
**Investment Return**. The position really did lose its value, and Investment Return is exactly the
income-statement term that should absorb it. #576's Investment half was never "write-offs are
mis-booked" — it was that a full negative return got booked when the capital *did* come back and the
user had simply not recorded the Sell.

So an investment write-off is modelled as **`sold` with a 0-proceeds Sell transaction**. No enum
change, no migration, and it satisfies the advisory in (7) rather than tripping it forever.

### 6. Terminating an Investment captures its settlement

`TerminatePositionDialog` — one generic dialog serving all four groups — gains a **settlement block**
for Investment, and writes the `sell`/`maturity` transaction in the **same transaction** as the
lifecycle flip. An explicit "no money came back — write this off" escape writes the 0-proceeds Sell of
(5) — the quantity still leaves the position, only the price is zero, so the cost basis closes out the
way a real sale would.

The block is **subtype-shaped**, not one "proceeds" scalar: it captures exactly what that subtype's
own transaction dialog would, because that is the only shape its transaction matrix accepts.

| Subtype | Terminal status | Settlement |
|---|---|---|
| Stock, MutualFund, Gold | `sold` | Sell — quantity × price_per_unit |
| Bond | `sold` / `matured` | Sell / Maturity |
| TimeDeposit | `matured` | Maturity — principal + interest, both `cash_out` |

A single scalar was the first shape considered and is rejected: a Sell is quantity-denominated (the
cost-basis replay reduces proportionally, so a Sell with no quantity leaves the basis stranded) and a
Maturity is a principal/interest pair. Deriving either from one number means fabricating the split.

Because the dialog's status dropdown is group-level, it also **narrows to the settleable pairs above**
for Investment — otherwise it offers a matured Stock and a sold TimeDeposit, combinations no
transaction can express.

The same matrix is enforced at the API: `UpdateInvestmentLifecycle` refuses a **transition into** an
unsupported pair, so the combination cannot be created by a raw call either. Refusing only the
transition — never a re-assertion — is what keeps a position that arrived on one via restore or import
fully editable, so its date and note can still be corrected; the way out is to reactivate and
terminate again properly, which is never blocked because `active` is not terminal. The dialog keeps a
current status the narrowing would otherwise drop, so it never blanks its own value.

**Defaults, and the one place a blank is deliberate.** The Sell's quantity comes from the *ledger*
(Σ buy − Σ sell), not the last snapshot, because the ledger is what the cost-basis replay reads — so
sizing the closing Sell from it is what drives the basis to zero. The price defaults to the last
marked price. Where there is none, it defaults to 0 **only if the position holds nothing** (0 × 0 is
the truthful settlement of an empty position, and the form must stay submittable — a required-blank
price would otherwise make such a position impossible to close at all). A position that *does* hold
something but has never been marked is left blank and required: what it sold for is exactly the
judgement this capture exists to take, and defaulting it to 0 would book a real sale as a total loss.

Rejected: a hard 400 unless a matching `sell`/`maturity` already exists in the termination month. It
blocks legitimate corrections, constrains the ordering of import and restore, and hands a
non-technical user an error with no obvious remedy. Capture-at-source closes the same hole without
any of that.

Also rejected: **dropping the terminate action for Investment entirely** and forcing termination
through a subtype-specific terminal transaction. It is the right instinct — it is already how
Bond/TimeDeposit maturity works (a Maturity flips the status itself, INV-LIFECYCLE-02) — but it breaks
on three counts. A Sell is not terminal (sells are partial by construction, so a closing Sell needs an
explicit "this closes the position" marker, which is the same capture problem relocated). The dialog
is the only surface for **un-terminate**, [[adr-0009]]'s correction affordance, and deleting a
Maturity does not reverse the flip. And restore/import can still land terminated-with-no-proceeds
positions (7), which would leave an affected household with no remedy at all.

The settlement is captured only on the **active → terminal** edge. Re-asserting a terminal status —
correcting a date or a note — books no second sale, and the repo rejects one. Deliberately *not*
deduplicated by "a sale already exists this month": several partial Sells in one month are legitimate,
so the position's own status is the only honest signal. The dialog does skip the capture when the
termination month already carries a sale the user recorded by hand, so it never offers to duplicate
one that is already there.

### 7. `unsettled_terminations` — a report-side advisory

Once (1) and (6) are in place, "terminated with no recorded proceeds" is only reachable by a path that
bypasses the dialog: **restore-from-backup** ([[adr-0036]] writes rows directly), import, or the raw
API. That is a genuine user-facing path — a household can inherit bad data from a restore with no way
to notice, which is the whole complaint in #576 — so the engine emits an advisory list of Investments
terminated without proceeds.

It is its **own** jsonb column on `monthly_reports`, mirroring `stale_positions`' shape rather than
extending it with a `reason`. "Stale" currently means one precise thing (no recent snapshot) and is
worth keeping precise.

### 8. Existing data is corrected by hand, not by a backfill migration

No backfill migration ships. The `write_offs` and `unsettled_terminations` columns are additive DDL;
the *data* correction — writing 0-value close snapshots for already-terminated Asset, Liability and
Receivable Positions — is applied manually to the two live households (the maintainer's and the
demo's), because there is no production deployment and no third-party user. This follows the same
call made on #575's orphan backfill. The correction is recorded in the release notes so self-hosters
restoring older data know why their historical months changed.

Writing those snapshots bumps `updated_at`, which moves the report staleness watermark and triggers
regeneration — the desired outcome here.

## Considered alternatives

- **Read-side suppression instead of close snapshots** — change `terminatedBefore` so a Position
  reads 0 from its termination month. No rows written, no correction pass, retroactive by
  regeneration alone; far less machinery. Rejected: the report would silently disagree with the
  snapshot the user entered, and Investment would keep writing rows, leaving two mechanisms meaning
  the same thing permanently.
- **Folding non-cash terminations into Asset Value Change** — no new column. Rejected: Asset Value
  Change is scoped to `property`/`vehicle` marks, and Liability/Receivable write-offs have no business
  in a term named `asset_*`. It would also cost the term the precision CONTEXT.md currently claims for
  it.
- **A gains/losses pair instead of one signed term** — separable for statistics. Rejected: doubles the
  DDL, PDF rows and stats surface for rare events, and nothing in [[adr-0048]] wants either side.
- **Flow-driven trigger** (fire when a terminal Position has no offsetting flow) — rejected: only
  Investment has flows to inspect, so it degenerates to status-driven for three groups anyway.
- **Adding `written_off` to the Investment status enum** — rejected per (5): Investment Return already
  absorbs a genuine total loss truthfully, so the status would encode a distinction the identity does
  not need.
- **Naming the term `non_cash_adjustments`** — rejected: under (3) Asset Value Change is also non-cash,
  so the name stops distinguishing. `non_cash_termination` is precise but clunky on a PDF row read by
  a non-technical household member. "Write-off" is the word they already know, and it covers a
  *written-off* receivable, a *forgiven* (i.e. written off by the creditor) liability and a *disposed*
  asset without explanation.

## Consequences

- **Engine version 4 → 5.** Every materialised report regenerates. This is the standing mechanism
  ([[adr-0048]] tail); historical months will change for any household holding a terminated Position,
  which is the point.
- **Write-Offs feeds no [[adr-0048]] statistic.** Cash-Flow Ratio, Passive-Income Ratio and Fund
  Resilience read Earned Income, Living Expenses and the return rate — none of which move once the
  residual is corrected. In particular a forgiven debt must **not** reach Earned Income, where it
  would inflate the savings rate in a month with no earnings.
- **No per-owner breakdown.** `user_breakdowns` carries `nw`, `earned_income` and `investment_return`;
  Asset Value Change is deliberately absent. Write-Offs is the same shape of term and follows that
  precedent.
- **TimeDeposit rollover was already correct and stays untouched.** A `rolled_to_new` maturity is also
  a termination with no cash leg, but both legs are already booked — the maturity books `cash_out`
  regardless of disposition, and the successor takes a matching rollover `cash_in` in the same month
  (issue #27). Recorded here so it is not re-opened as a gap.
- **Restore-from-backup remains the one path that can introduce unsettled terminations.** (7) makes it
  visible rather than silent; it does not prevent it.
- **A full-fidelity backup taken before the correction carries the old snapshots verbatim**, so
  restoring one reintroduces the wrong months. Same residual as #575's orphan rows.
