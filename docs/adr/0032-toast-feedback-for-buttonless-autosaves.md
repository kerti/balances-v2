# Toast feedback for buttonless autosaves

A handful of controls write to the server **on change, with no Save button**: the position Tag
dropdown on each detail screen ([[adr-0028]]), and the Language and Appearance selects in Settings
([[adr-0026]]). These "buttonless" interactions had no positive confirmation that the write landed —
success was silent, and failure showed a small inline `<p>` below the control that is easy to miss.
We add a single app-wide **toast** surface (**sonner**) and route success/error feedback for those
autosaving controls through it (issue #54).

## Why now

The autosave pattern is now established and repeated: the Tag dropdown copied the
Language/Appearance selects deliberately, and each one fire-and-forgets a mutation. Three of them
exist today and more will follow (any preference that should persist instantly wants the same
shape). A button-driven form gives its own feedback — the button returns to rest, the dialog closes,
the row appears — but an autosaving select gives none. Without a confirmation a non-technical user
([[feedback-audience-non-technical]]) cannot tell whether their choice stuck, and the inline error
copy was both inconsistent (each card rolled its own `<p>`) and spatially detached from where
attention sits after picking from a dropdown.

## The decision

### sonner, not a hand-rolled Radix Toast

sonner is shadcn's canonical toast and ships the viewport, stacking, auto-dismiss, and animation out
of the box. Building the equivalent on the `radix-ui` Toast primitive (already a dependency) is more
code for no benefit at our scale. One `<Toaster>` mounts at the root inside the existing
`ThemeProvider`/`QueryClientProvider`; a thin wrapper (`components/ui/sonner.tsx`) wires sonner's
palette to the app's own theme axis (`useTheme`, not next-themes) and maps its CSS hooks onto the
`index.css` custom properties so toasts inherit the shared popover surface.

### Success wears the brand accent; errors stay destructive

Success toasts use `--primary` (the brand accent) so the confirmation pops rather than blending into
the page; errors use the destructive palette. Error copy is resolved through the existing
`errorMessage()` helper ([[adr-0027]]), so toasts speak the same localized envelope strings as the
dialogs do — no new error vocabulary.

### Scope: only the buttonless surfaces

Only the three autosaving selects move to toast-and-rollback (optimistic switch, revert on failure
so the control keeps showing the truth). Button-driven forms — nickname, reporting currency, FX
rates, Tag create/rename/delete — keep their inline error `<p>`: the button already anchors feedback,
and converting them is out of scope for #54. The multi-currency toggle is left as-is for now; its
error display is shared with the sibling Save button, so moving just the toggle would split one
card's feedback across two mechanisms.

### Confirmation copy renders in the target language

The Language toast resolves its string with an explicit `lng` override (`t('language.saved',
{ lng: next })`) so it greets the user in the language they just picked, not whichever one i18next has
finished switching to when the PATCH resolves.

## Consequences

- One new runtime dependency (`sonner`) and one mounted `<Toaster>`. New autosaving controls get
  confirmation for free by calling `toast.success` / `toast.error` at the call site.
- Per-namespace `*.saved` copy (`settings.language.saved`, `settings.theme.saved`, `tags.field.saved`)
  lands in both locale catalogs; error copy continues to flow through `errorMessage()`.
- The three buttonless selects no longer render an inline error `<p>`; their failures and successes
  are toast-only. Button-driven forms are unchanged, so feedback is briefly inconsistent across the
  app until (if) those are migrated too — an accepted, bounded inconsistency.
- Toasts are not a tested surface in the unit runner (component testing is still deferred,
  [[adr-0021]]); the change is verified by build + manual eyeball.
