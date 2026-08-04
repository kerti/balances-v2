# Tracking Changes — when the books' coverage moves

A Position's value can enter or leave the Household's books without anything being earned, spent or
invested: a spouse's accounts joining at marriage, a dormant passbook finally entered years into
tracking, a departing member's Positions leaving. The comprehensive income identity ([[adr-0008]]) had
no term for it, so the whole amount landed in the derived Living Expenses residual — reading as a huge
*negative* month's spending on the way in, and as spending on the way out (issue #591).

This is the same *class* of defect [[adr-0052]] introduced the Write-Off term for — a one-sided
net-worth movement with no income-statement counterpart. That one fires off a Position dying; this one
fires off the **edge of the book moving**. This ADR extends [[adr-0008]]'s identity and [[adr-0009]]'s
lifecycle rules; neither is superseded, and [[adr-0052]] is amended in one place (§5, below).

## What the engine actually cannot see

The tempting fix — "suppress a Position's birth month the way the investment-return pass does" — is
wrong, and understanding why is the whole design.

`INV-FINANCE-23` suppresses the **Investment Return** line in a position's birth month
(`monthly_reports_engine.go`, `!okPrev ⇒ continue`). It does **not** suppress the position's
contribution to net worth, and it was never the thing protecting the residual. What protects the
residual in a normal acquisition is that the acquisition is *funded from tracked wealth*, so the other
leg moves too:

| | month M | ΔNet Worth | residual today |
|---|---|---|---|
| **Acquisition** — new account opened, 1,000 moved from an existing one | new +1,000, old −1,000 | 0 | correct |
| **Tracking Change** — account held for years, first entered | new +1,000, nothing else moves | +1,000 | **−1,000, wrong** |

Both rows present to the engine as *a Position with a first Snapshot and no prior value* — literally
the same `!okPrev` branch. A blanket birth-month term would fix row 2 and break row 1 by the full
amount, turning every genuine acquisition into phantom spending.

So **Tracking Change is declared, never inferred.** Only the Household knows which happened.

Two consequences of the same reasoning, worth stating because both contradict the issue as filed:

- **All four groups are affected on the way in**, not three. An Investment onboarded by Snapshot with
  no Buy is one-sided in exactly the way a bank account is; its return line is suppressed and nothing
  else absorbs it. Investment's guard was never protection.
- **A household that onboards everything at once already has no defect.** `baseline := idx == minIdx`
  suppresses the income statement wholesale in the Household's first Snapshot month. #591 is precisely
  the *post-baseline* birth — which is also why `acquired` is the right default (below).

## The decision

### 1. A new signed term: Tracking Change

**Tracking Change**: the value a Position brought into, or took out of, the Household's books when what
changed was the books' coverage rather than earning, spending or investing. The identity becomes:

```
ΔNet Worth = Earned Income + Investment Return + Asset Value Change + Write-Offs
             + Tracking Changes − Living Expenses
```

or as the engine computes the residual:

```
living_expenses = earned_income + investment_return + asset_value_change + write_offs
                  + tracking_changes − ΔNW
```

**One signed term covering both directions**, not a birth-only term and not an in/out pair. [[adr-0052]]
§4 already settled this argument for Write-Off — a *correction* term whose job is keeping the residual
honest, not a category a household budgets against; a month containing both directions nets toward zero
on the line, and that is solved by rendering the constituent Positions beneath it. The reasoning
transfers verbatim, and the identity is the single most expensive thing here to amend twice: every
change is an `engine_version` bump and a full regeneration of every materialised report.

Sign follows the effect on net worth, so a departing member's Liability contributes positively — the
same negation the net-worth pass applies to a liability's balance.

### 2. It is named for the mechanism, not for any of its stories

Three stories share one mechanism, and no story covers the other two:

| Story | Did the Household own it before? |
|---|---|
| Dormant passbook finally entered | **Yes** — only coverage changed |
| Spouse's accounts joining at marriage | **No** — the Household itself changed |
| Departing member's Positions leaving | Reverse of the above |

"Opening balance" is false — the book is years old. "Newly tracked" alone is honest but has no way to
name the exit. "Transfer" invites the one misreading that matters: a non-technical household member
reads `Transfers in/out  +1,000` as *money I moved between my own accounts* — an entirely ordinary
thing to do, and the one concept this app deliberately does not track ([[adr-0003]]). "Capital contributions
/ draws" is accurate partnership accounting and bounces off the audience, the same ground [[adr-0052]]
rejected `non_cash_termination` on.

**Tracking Change** states the ledger fact and claims nothing about ownership, so it is literally true
in all three. Its acknowledged weakness: for a departing member it *undersells* — the Household really
did lose that wealth and the line says something milder. Accepted, because the line's job is to say
"this movement is not spending", and it does that.

### 3. Entry is declared by an entry type; exit is declared by a terminal status

Exit follows [[adr-0052]] mechanically — a new terminal status `untracked`, joining `nonCashTerminal`
so the existing write-off machinery shape applies. Entry has no existing concept (a Position's birth is
implicit today: the row exists, the first Snapshot lands), so all four position tables gain:

```sql
entry_type text NOT NULL DEFAULT 'acquired'
  CHECK (entry_type IN ('acquired', 'newly_tracked'))
```

**Default `acquired`.** Post-baseline births skew heavily toward acquisition, and the mass-onboarding
case is already covered by the baseline suppression. The wrong default is expensive in the common case
and cheap in the rare one.

**Captured as two radios, not a checkbox** — an unchecked box says nothing, and this needs an
affirmative answer:

> **Where did this come from?**
> ◉ We funded it with money already tracked here
> ○ We already had it, or it came into the household

**Editable afterwards on the detail page.** This is load-bearing, not a convenience: it means a
Household that gets it wrong sees a wrong Living Expenses figure, flips one control, and the report
regenerates. Same correction affordance as [[adr-0009]]'s un-terminate, and it is what makes a bad
month inherited from a restore or an import recoverable.

### 4. The term fires at the first Snapshot month, and there is no entry boundary

On the way in it fires at the Position's **first Snapshot month**; on the way out at its termination
month. This is forced, not chosen: the Position enters `nwTotal` at its first carried value, so the term
must fire there or the identity stays open — the same reason Write-Off must fire at `terminatesAt`.

It follows that the entry-side computation needs a different shape from the two existing loops, which
difference two `fx.carried` calls and bail on `!okPrev`. Entry **is** the `!okPrev` case: the term is
`now − 0`, taken from the first Snapshot directly.

A Position marked `newly_tracked` still contributes to net worth for every month it has Snapshots,
including months before the Household had any relationship to it. Rejected: an `entry_date` boundary
mirroring `terminated_at`, with pre-entry Snapshots visible but excluded from net worth. The Household
already controls this by choosing what history to enter — a couple who don't want pre-marriage wealth in
their history enter Snapshots from the marriage month. The boundary is machinery to override a decision
the user already made deliberately, it is the shape [[adr-0052]] rejected in its own alternatives
("the report would silently disagree with the Snapshot the user entered"), and the symmetry with
`terminated_at` is superficial: termination is a fact about the *Position*, whereas an entry boundary
would be a fact about the *Household's relationship* to it — which `entry_type` already encodes without
a date, a second suppression rule, or an interaction with carry-forward.

The honest cost: a Household entering a new member's full history will see its own past net worth rise,
and the Tracking Change line will sit in the earliest month of that history rather than the month they
married.

### 5. Investment takes `untracked`, amending ADR-0052 §5

[[adr-0052]] §5 refused Investment a write-off status because a genuine total loss *is* truthfully a
negative Investment Return, so the status would encode a distinction the identity does not need. That
argument does not transfer to the exit side of a Tracking Change. A departing member's stock portfolio
did not lose its value — booking it as a large negative return is the same class of falsehood #576
complained about. So `untracked` is available to every group, and is the only terminal status that is.

`untracked` is therefore **exempt from [[adr-0052]] §6's settlement capture**: the terminate dialog
demands a Sell/Maturity for an Investment's terminal statuses, and nothing was sold here. The API's
transition matrix must admit it without a settlement.

### 6. It feeds no statistic and no per-owner breakdown

Tracking Changes must never reach Earned Income. An onboarded Position can carry many years' worth of
accumulated wealth, so booking it as income would drive the Cash-Flow Ratio to near 100% in a month with
no earnings at all and poison the trailing-12 smoothing for a year afterwards. This also rules out the
no-code-change workaround of recording a marriage as an `Income` event of category `Gift` — the same
ground [[adr-0052]] rules a forgiven debt off Earned Income on, and the figures here are larger.

No `user_breakdowns` entry, following Write-Offs' precedent (`nw`, `earned_income`,
`investment_return` only).

### 7. Existing data is corrected by hand, not by a backfill migration

Following [[adr-0052]] §8 and #575: the `tracking_changes` column and `entry_type` are additive DDL, and
the *data* — flipping already-onboarded Positions to `newly_tracked` — is applied manually to the two
live Households, because there is no production deployment and no third-party user. `DEFAULT 'acquired'`
means existing rows keep today's behaviour until a Household says otherwise, so no month changes without
a deliberate act.

## Consequences

- **`engine_version` 5 → 6.** Every materialised report regenerates — the standing mechanism
  ([[adr-0048]] tail).
- **A corrected residual is not a validated one.** This removes one class of one-sided movement. A month
  can still be wrong from unrecorded income, a terminal status that asserts a cash leg which never
  happened, or a settlement recorded a month late — none of which this term detects or is meant to.
- **The identity now has three correction terms** (Asset Value Change, Write-Offs, Tracking Changes)
  against two tracked/derived ones. Each pulls a specific non-cash movement out of the residual; the
  claim CONTEXT.md makes for the residual being a genuine cash-spending proxy depends on all three.
- **A Position's entry type is a new thing backup/restore and import must carry.** Dropping it on
  restore silently defaults a `newly_tracked` Position back to `acquired` and reintroduces the wrong
  month — the same residual as [[adr-0052]]'s and #575's.
- **Nothing detects a mis-declared entry.** Unlike `unsettled_terminations` ([[adr-0052]] §7), there is
  no fact the engine can check: a one-sided birth is indistinguishable from an acquisition whose funding
  the Household has not snapshotted yet. An advisory here would be a heuristic firing on wrong months,
  so none ships. The remedy is the editable control in §3.
