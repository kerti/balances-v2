# Feedback and safety: replaces are archived, single-item writes can be undone, one toast per write

Round (c) of the 2026-11-04 UI audit (INC-19/23/24/26/27, PAT-04, #805). The app had no rule for what
a user sees after a write, and no way back from one. Some writes said nothing, the confirm dialogs
told a non-technical household to "undo via the database", and replacing a month's value destroyed
the old one in some flows (bulk entry, import, Edit, which upsert or `PATCH` in place) but archived it in
another (close, [[adr-0052]] §2). This ADR sets one rule for recoverability and one for feedback.
It amends [[adr-0046]]'s "one overwrite rule": the Record month replace path is no longer a `PATCH`
in place.

## Decisions

1. **Replacing a Snapshot archives, never overwrites.** Every flow that changes an existing
   Snapshot's values (Edit, a Record month replace, Move, bulk entry, import) soft-deletes the live
   row and inserts the new values in one transaction. This is [[adr-0052]] §2's close mechanism,
   generalised, **except for the link**: `supersedes` keeps its one meaning, "a close row displaced
   this", and an ordinary replace leaves it null. The write's response returns the displaced row's
   id, and the Undo toast carries it. Storing the link would be wrong twice over: a compacted backup
   carries any archived row a live row supersedes (so it would carry every replaced month), and a
   twice-edited month would leave the carried row pointing at one that isn't, breaking the file's
   closed foreign keys. `PATCH` stays the Edit endpoint; only its effect changes. Consequences: a
   Snapshot's id changes on every edit (nothing references snapshot ids, and the client refetches),
   and the displaced row is an ordinary soft-deleted ("Recycle Bin") row, carried only in
   full-fidelity backups.

   **No purge.** Archived rows are kept, like every soft-deleted row. At household scale they cost
   tens of KB a year and nothing at query time (live reads go through the `deleted_at IS NULL`
   partial index), and they are the only record of what a month said before. A timed hard-delete
   was rejected: the undo window lives in the client, it would make replaced values less durable
   than deleted ones, and it must never touch a row a live close row supersedes. If size ever
   matters, the tool is a deliberate retention purge (backlog), not a timer.

2. **A single-item delete or replace can be undone from its toast.** Undo clears the archived row's
   `deleted_at` and, for a replace, soft-deletes the row that displaced it. Snapshot and transaction
   undos regenerate the affected months' reports. A replace undo names both rows (the one to
   restore and the one to archive) and refuses unless the second is still the live row for that
   slot. If the slot has been taken in the meantime
   (same month, tag name or rate date), undo refuses with an error code and its own copy. A bulk
   save has no Undo: restoring N rows with conflicts costs more than this uncommon mistake is worth,
   and its displaced values are archived anyway.

3. **Three confirmation tiers.**
   | Tier | Items | Before | After |
   |---|---|---|---|
   | Undo only | Snapshot, transaction, income row, exchange rate, inflation rate, tag | nothing | toast with **Undo** |
   | Confirm, then Undo | Position | red `ConfirmDialog`, one template: "Delete {name}? It disappears from lists and reports." | toast with **Undo** |
   | Type to confirm | Delete household, Restore (erase) | type the name / `ERASE` | no Undo: irreversible |

   A position keeps its dialog because one click hides years of history and a toast is brief. The
   "undo via the database" sentence goes everywhere. A toast with an action lasts **8 s** and pauses
   on hover; a plain toast keeps the 4 s default.

4. **Every write the user starts ends with exactly one toast.** Deletes and replaces say what
   happened and offer Undo. Creates and edits say "Saved"/"Added", with no Undo (undoing a create is
   a delete). A bulk save says "Saved 7 · 2 replaced earlier values". Long or background work
   (import, export, rebuild, restore) shows a loading toast that resolves to success or error. The
   rejected alternative, "toast only when the result isn't visible", is today's rule and the reason
   the audit found it inconsistent.

5. **Errors go where the user is looking.** With a form open, errors show inline: client validation
   per field on blur or submit, server error codes mapped to their field, or a form-level message
   when no field fits. Submit is never disabled without a reason. With no form open (a failed
   delete, undo or export), a red toast shows the localised message for the server's code, never a
   generic "Something went wrong" when a code came back.

6. **After a bulk save, the toast carries the next step.** Saving navigates back, as today, landing
   at the top of the page. When another group still has stale Positions that month, the toast's action
   is **Next: {group} ({n}) →**. The dashboard's stale-warning link ([[adr-0046]], #792) stays the
   persistent way in, so the toast is a shortcut, not the only route.

7. **Settings saves by kind of control.** Choices (select, switch, theme) save on change with a
   toast. Typed values (names, reporting currency, inflation) keep an explicit **Save**, enabled
   only when dirty; Enter saves, Esc reverts, and a toast confirms. Autosaving typed text was
   rejected: a half-typed currency code or an inflation typo would silently change every report.

## Amendment (2026-11-10): tags are not Undo-tier

§3 listed the tag in the Undo-only tier, but a tag has no undo. `DeleteTag` clears every item's
`tag_id`, and soft-deleting the tag could not put those assignments back (INV-SOFT-DELETE-06), so
building an Undo would mean storing every cleared assignment for a toast's lifetime. A tag delete
keeps a red `ConfirmDialog` ("Delete tag {name}? It's removed from every item that has it.") and
then a plain "Tag deleted" toast with no action. The tier table's Undo-only row loses "tag" and the
Confirm-then-Undo row gains a sibling, Confirm-then-plain-toast, holding the tag.
