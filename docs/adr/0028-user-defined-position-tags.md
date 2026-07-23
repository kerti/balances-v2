# User-defined position tags

A **Tag** is a household-defined label a User can attach to any Position to group it on a
breakdown report. Each Position carries **at most one** Tag (a nullable FK); Positions with none
fall into an **Untagged** bucket. A new report sums Position value by Tag, per currency, so the
household can answer "how much sits behind each grouping I care about" — by bank, by goal, by risk
bucket, by anything they choose to name. Tags are orthogonal to every existing domain field
(notably the bank-account / time-deposit `bank_name`), carry no built-in financial meaning, and are
free-add only (no seed list).

## Why now

Issue #28 began as "reshape Banks from free text into a lookup, attach it to positions, and report
totals per institution." The general need underneath it is **customized asset grouping**: a User
wants to slice their positions along an axis the app does not model — which institution holds a
position, which life goal it serves, which risk bucket it belongs to — and read the totals and
proportions for that slice.

Baking any one such axis into the schema (a bank lookup, a custodian FK, a goal enum) would solve a
single case while inviting the next one as another migration. Instead the household gets a
**neutral grouping primitive** and supplies the meaning. "By bank," "by goal," "by risk" all become
Tag values the User names — no fixed taxonomy, no financial semantics in the model. Because a Tag
asserts nothing about *where value is held* or *what a position is*, it composes cleanly with every
group without special-casing.

## The decision

### A Tag is a household-scoped, soft-deleted lookup

```sql
CREATE TABLE tags (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID        NOT NULL REFERENCES households(id),
    name         TEXT        NOT NULL,
    color        TEXT        NOT NULL,              -- one of a fixed swatch palette
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);
-- name unique per household among the living
CREATE UNIQUE INDEX tags_household_name_live
    ON tags (household_id, lower(name)) WHERE deleted_at IS NULL;
```

`color` is required at create — the User picks from a fixed swatch palette so a tag keeps a stable
hue across the pie and table. Free-add only: there is no seed list. Soft-delete follows
[[adr-0007]]; a deleted Tag's FK references go NULL-at-read (the Position falls back to Untagged)
rather than cascading a hard delete.

### One nullable `tag_id` on each shared Position table

The FK lives on the four shared Position parents, not the subtype extension tables:

```sql
ALTER TABLE assets       ADD COLUMN tag_id UUID REFERENCES tags(id);
ALTER TABLE liabilities  ADD COLUMN tag_id UUID REFERENCES tags(id);
ALTER TABLE receivables  ADD COLUMN tag_id UUID REFERENCES tags(id);
ALTER TABLE investments  ADD COLUMN tag_id UUID REFERENCES tags(id);
```

This covers all ten groups (three asset subtypes, two liability subtypes, receivable, five
investment subtypes) with four columns, because the subtype rows hang off these four parents.
**Income is excluded** — it is a flow event, not a Position (CONTEXT "Income is a flat flow event"),
and net-worth grouping is a position concept.

### Single tag, not many — for clean proportions

A Position carries **at most one** Tag. The deciding reason is the report: with multiple tags per
Position, "proportion by tag" double-counts and the slices sum past 100%, so a pie stops meaning
anything. One optional FK makes Tags a **partition** — every Position lands in exactly one slice
(its Tag, or Untagged), proportions sum to 100%, and the pie is well-defined without a
normalisation rule. Multi-tag is a deferred escalation (a `position_tags` join table) if a real
multi-membership need appears; there is no signal for it yet, and YAGNI applies.

### Assignment is a dedicated endpoint, not a create/update field

Because a Tag is orthogonal to a Position's identity, assignment is a single unified endpoint —
`PUT /api/tags/assignments` with `{group, position_id, tag_id|null}` (`group` ∈ asset / liability
/ receivable / investment) — rather than a `tag_id` field threaded through all eleven position
create/update request shapes. This keeps the existing create/update flows untouched, gives the
tenancy check one home, and lets a future "drag a position between tags" surface reuse the same
route. The trade-off is that creating a Position *with* a Tag is two calls (create, then assign)
instead of one atomic insert; for a cosmetic grouping with no financial-integrity stake that is an
acceptable seam. The current `tag_id` value rides back on every Position read for nothing extra:
the position queries are `SELECT *`, so the new column surfaces on GET/LIST automatically and the
edit dialog preselects from it.

### Tenancy: belt + suspenders, as everywhere

Assigning a Tag validates `tag.household_id == position.household_id` in SQL, not just middleware —
the same rule every position-touching query already follows. The `UPDATE … SET tag_id` filters the
Position by `household_id` and guards the Tag with a `household_id`-scoped subquery, so a Tag or
Position from another household is `ErrNotFound`, never a silent cross-tenant link.

### The report: Σ value by Tag, per currency

A new household-scoped aggregate endpoint returns, per `(tag, currency)`, the sum of each
Position's **most recent snapshot value with `year_month ≤ now`** (the same carry-forward valuation
net worth uses, CONTEXT "Net Worth"). Conventions:

- **Per currency, no FX** — matching the list/home-screen convention; a multi-currency household
  sees one breakdown per currency rather than a converted total.
- **Liabilities are their own negative slice**, not netted into a tag's assets — a tag mixing a
  mortgage and a savings account should show both magnitudes, not their difference.
- **Untagged** is a real bucket in the output, so proportions are honest.
- Terminated Positions follow the net-worth rule: they contribute only for months ≤ their
  `terminated_at`, so a sold/closed Position drops out of the current-value breakdown.

UI: a dedicated `/tags` route (flat nav group, like Receivables / Income) renders a pie
(proportion) + table (sums) per currency. Tag management (create / rename / recolor / delete) is a
card (`TagsCard`) merged onto the **same `/tags` page** rather than a separate Settings subpage — a
"manage" surface and its own report were needless duplication split across two places — mirroring the
locale + theme cards ([[adr-0026]]) in tone. The report leads and management sits at the bottom (see
Presentation / UX). A single-select Tag dropdown defaulting to "No tag" sits in every Position
Create/Edit dialog; on save the dialog fires the position mutation and, if the selection changed, the
assign call.

## Presentation / UX

**Page order (both form factors).** The `/tags` page leads with the **report** (donut + breakdown
per currency) and puts the **management card at the bottom**. The report is what a returning
household member [[adr-0025]] comes back for; creating or recolouring a tag is the occasional setup
task, so it sits below rather than pushing the report down the page. `TagsScreen` is the container:
it owns the query and the per-currency **pie-inclusion state** (which slices are checked), and hands
each currency's projection to `TagBreakdownSection`.

Per [[adr-0050]] (mobile–web layout divergence doctrine), the breakdown **diverges its mobile layout**
from the web layout: the wide holdings · liabilities · net table horizontally scrolls on a phone,
hiding the very sums the report exists to show. `TagBreakdownSection` delegates only the breakdown
body to one of two renderers, picked at runtime by `useIsMobile()` (the single 768px boolean); one
tree is ever in the DOM. Both consume the same `CurrencyBreakdown` projection and the same
`isChecked`/`toggle` handlers (keyed by `cellKey` — the tag id or `untagged`), so the pie-inclusion
behaviour can't fork per renderer, and both sit under the same `tag-breakdown-<currency>` testid.

- **`TagBreakdownTable`** (≥768px) keeps the wide table: a per-row pie-inclusion checkbox, the tag
  badge, right-aligned `tabular-nums` holdings / liabilities (negative, destructive) / net columns,
  and a **Total** footer row.
- **`TagBreakdownCards`** (<768px) applies the doctrine's **"wide table → stacked cards"** transform:
  one card per tag with the **net value promoted to the card headline** (`tabular-nums`, readable with
  no horizontal scroll), the checkbox + tag badge on the top line, and holdings / liabilities stacked
  below as label→value pairs. The top line is a `<label>` wrapping the checkbox at the a11y floor
  (`min-h-11`, ≥44px) — the desktop's bare 16px checkbox would miss the floor on a phone. A distinct,
  checkbox-less **Total** card (muted fill) closes the stack, since a total is a summary, not a
  toggleable slice.

The shared **pie** is not a renderer split — only its legend placement is a cosmetic prop: to the
**right** of the donut on desktop, and **dropped entirely** on phones (`legendPosition="none"`), where
the breakdown cards below already carry the tag badge + colour and so double as the legend. Its
absence lets the donut's container tighten (no reserved legend row), keeping the mobile vertical
rhythm compact. The management card stays **single-layout** — it already reflows (flex-wrap, no table)
without breaking the a11y floor, so it earns no divergent renderer per the doctrine's bar.

One `@smoke` Playwright spec (`tags-mobile.spec.ts`) asserts the correct renderer mounts at mobile vs
desktop width and the net value stays reachable; the desktop assign/report round-trip stays in
`tags.spec.ts`, and renderer conformance in `TagBreakdownSection.test.tsx`.

## Out of scope

- **The bank lookup / institution framing.** `bank_name` on bank accounts and time deposits stays
  free text and untouched; Tags do not replace it. The original "banks as a lookup" idea is
  superseded, not deferred.
- **Multi-tag per Position** (join table) — deferred until a real need appears.
- **Tagging Income** — excluded by the position/flow-event split.

## Consequences

- One migration (00025): the `tags` table + four nullable `tag_id` columns. No backfill — every
  existing Position reads as Untagged, and `SELECT *` surfaces the column on every Position read
  without touching the existing create/update/select queries.
- The Position Create/Edit dialogs across all ten groups gain a shared Tag-select component;
  Settings gains a Tags card; a new `/tags` report screen and one nav entry land.
- Because the FK is nullable and defaults NULL, the feature is fully additive — no existing flow
  changes behaviour until a User creates and assigns a Tag.
