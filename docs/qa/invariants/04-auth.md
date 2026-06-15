# Zone: AUTH

The other half of the access-control threat model. TENANCY guards **which rows**
an authenticated household sees; AUTH guards **who you are** at the door, and
establishes the `household_id` every TENANCY filter then trusts. A break here is
the same finance leak TENANCY prevents, entered one layer earlier. Two security
hinges: the OAuth `state`/`session` cookies that authenticate a browser, and the
invitation flow that decides which household a brand-new user joins — a forwarded
invite link must never let an unintended Google account into someone else's
household. Code lives in `internal/auth/`: `session.go`
(`RequireAuth`/`SessionMiddleware`), `google.go` (OAuth + `randomState`),
`invitations.go`, `handlers.go` (callback + `bootstrapNewUser`/`createFounder`).

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-AUTH-01 | An unauthenticated request to a protected route is rejected with 401 by `RequireAuth` before the handler runs | ADR-0017, ADR-0005 | Critical |
| INV-AUTH-02 | The OAuth `state` is random and the callback rejects (400) any request whose `state` query param does not match the state cookie set at start (CSRF guard) | ADR-0017 | Critical |
| INV-AUTH-03 | A session is identified by a random opaque cookie value (HttpOnly, SameSite=Lax, Secure in prod); an unknown or expired session never authenticates, and a valid one attaches the user and slides the TTL | ADR-0017 | Critical |
| INV-AUTH-04 | Logout deletes the session row and clears the cookie, and is idempotent when no cookie is present | ADR-0017 | High |
| INV-AUTH-05 | First sign-in with no matching `google_sub` and no invitation bootstraps a brand-new household for the founder | ADR-0017 | High |
| INV-AUTH-06 | An invitation token is random, single-use, and expiring; an unknown, already-used, or expired token is rejected | ADR-0017 | Critical |
| INV-AUTH-07 | Accepting a valid invitation binds the new user to **only** the inviting household (not a new one) and marks the invitation used | ADR-0017, ADR-0005 | Critical |
| INV-AUTH-08 | Invitation acceptance requires the Google-supplied email to match `invited_email` (forwarded-link guard); a mismatch is rejected and leaves the invitation unconsumed | ADR-0017 | Critical |
| INV-AUTH-09 | The pre-auth language pick is display-only: it rides the OAuth round-trip in a short-lived `oauth_locale` cookie (set at start only for a supported BCP47 `?lng=`, cleared at the callback) and never PATCHes an account | ADR-0035 | High |
| INV-AUTH-10 | A brand-new account's `locale` is seeded server-side at birth from the `oauth_locale` hint, falling back to `en-GB` when the hint is absent or unsupported — for both the founder and the invited-member paths | ADR-0035 | High |
| INV-AUTH-11 | The invitation accept URL carries the inviter's locale as `?lng=` (a direct backend `/start` link), so an invitee inherits the household language by default; override is available later in Settings | ADR-0035 | Medium |
