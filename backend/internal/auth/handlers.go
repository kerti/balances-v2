package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/email"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

const (
	oauthStateCookieName  = "oauth_state"
	oauthInviteCookieName = "oauth_invite"
	oauthLocaleCookieName = "oauth_locale"
	invitationTTL         = 72 * time.Hour
	// passwordResetTTL is deliberately short relative to the 72h invite: a reset
	// link is a live key to an existing account, so its window of replay is kept
	// tight (#282, ADR-0039).
	passwordResetTTL = 1 * time.Hour
)

// defaultSeedLocale is the lingua-franca fallback for a brand-new account when
// the pre-auth picker sent no usable hint (ADR-0035). An Indonesian browser is
// routed to id-ID by the picker's navigator pre-fill, so this only wins for an
// unknown visitor whose browser signals nothing supported.
const defaultSeedLocale = "en-GB"

// resolveSeedLocale maps a pre-auth oauth_locale hint to the locale a new
// account is born with: the hint if it's a supported BCP47 form, else the
// en-GB default. Display-only and never persisted for an existing user — only
// createFounder / bootstrapNewUser consult it, and only at account birth.
func resolveSeedLocale(hint string) string {
	if _, ok := supportedLocales[hint]; ok {
		return hint
	}
	return defaultSeedLocale
}

type Config struct {
	// GoogleEnabled / LocalEnabled select the live identity providers (ADR-0039).
	// When GoogleEnabled is false, New constructs no Google OAuth client and makes
	// no OIDC discovery call, so a local-only self-host needs no Google creds.
	GoogleEnabled bool
	LocalEnabled  bool
	// EmailEnabled mirrors the boot EMAIL_ENABLED (ADR-0037). It gates the emailed
	// password-reset path (#282): with mail off there is no way to deliver a reset
	// link, so the methods endpoint advertises reset as unavailable and the request
	// endpoint is a no-op (the mail-off reset paths are #283 reactivation / #284
	// operator CLI). The Mailer itself is already a NoopMailer when this is false.
	EmailEnabled bool
	// Pool backs the multi-statement local-invite accept (#281), which consumes
	// the invitation and creates the User+credential in one transaction so a
	// failure can't burn the single-use link without minting an account.
	Pool         *pgxpool.Pool
	Google       GoogleConfig
	SessionTTL   time.Duration
	CookieSecure bool
	FrontendURL  string
	BackendURL   string
	EmailFrom    string
	Mailer       email.Mailer
}

type Handlers struct {
	q             *db.Queries
	pool          *pgxpool.Pool
	googleOAuth   googleOAuthClient
	googleEnabled bool
	localEnabled  bool
	emailEnabled  bool
	limiter       *loginLimiter
	mailer        email.Mailer
	validate      *validator.Validate
	sessionTTL    time.Duration
	cookieSecure  bool
	frontendURL   string
	backendURL    string
	emailFrom     string
	// dispatch runs a background task. In production it spawns a goroutine; the
	// reset-request path uses it to send the email *off* the request goroutine so
	// the SMTP round-trip never lands in the response timing (the no-enumeration
	// timing guarantee, #282). Tests swap in a synchronous variant for determinism.
	dispatch func(func())
}

func New(ctx context.Context, q *db.Queries, cfg Config) (*Handlers, error) {
	if !cfg.GoogleEnabled && !cfg.LocalEnabled {
		return nil, errors.New("auth: no provider enabled (set GoogleEnabled and/or LocalEnabled)")
	}
	// Construct the Google OAuth client only when Google is enabled — the
	// newGoogleOAuth call performs OIDC discovery (an outbound network call), so a
	// local-only boot must skip it entirely (ADR-0039). googleOAuth stays nil and
	// the Google routes are never mounted.
	var g googleOAuthClient
	if cfg.GoogleEnabled {
		concrete, err := newGoogleOAuth(ctx, cfg.Google)
		if err != nil {
			return nil, err
		}
		g = concrete
	}
	if cfg.Mailer == nil {
		return nil, errors.New("auth: mailer is required")
	}
	if cfg.BackendURL == "" {
		return nil, errors.New("auth: backend url is required")
	}
	return &Handlers{
		q:             q,
		pool:          cfg.Pool,
		googleOAuth:   g,
		googleEnabled: cfg.GoogleEnabled,
		localEnabled:  cfg.LocalEnabled,
		emailEnabled:  cfg.EmailEnabled,
		limiter:       newLoginLimiter(),
		mailer:        cfg.Mailer,
		validate:      httperr.NewValidator(),
		sessionTTL:    cfg.SessionTTL,
		cookieSecure:  cfg.CookieSecure,
		frontendURL:   cfg.FrontendURL,
		backendURL:    cfg.BackendURL,
		emailFrom:     cfg.EmailFrom,
		dispatch:      func(fn func()) { go fn() },
	}, nil
}

// Mount registers auth routes under the provided router. The caller is expected
// to apply SessionMiddleware at a higher level so /api/me sees the user.
func (h *Handlers) Mount(r chi.Router) {
	// The SPA reads which providers are live from here before rendering the
	// sign-in screen (ADR-0039) — always available, even pre-auth.
	r.Get("/auth/methods", h.handleAuthMethods)
	if h.googleEnabled {
		r.Get("/auth/google/start", h.handleStart)
		r.Get("/auth/google/callback", h.handleCallback)
	}
	if h.localEnabled {
		r.Post("/auth/local/register", h.handleLocalRegister)
		r.Post("/auth/local/login", h.handleLocalLogin)
		// Local invite accept (ADR-0039/#281): the invitee follows their link and
		// sets a password — possession of the single-use link IS the email proof,
		// so this path mints a session directly and never re-enters the ADR-0038
		// onboarding gate. Public (the invitee has no session yet); the hashed
		// token in the body is the credential.
		r.Get("/auth/local/invite", h.handleLocalInvitePreview)
		r.Post("/auth/local/invite/accept", h.handleLocalInviteAccept)
		// Emailed self-service password reset (ADR-0039/#282): a member requests a
		// reset, receives a single-use emailed token, and sets a new password. The
		// request endpoint never reveals whether the email maps to an account (no
		// enumeration); the GET preview validates a token without consuming it; the
		// set endpoint consumes the token, replaces the credential, revokes the
		// member's other sessions, and signs them in. Public (the holder has no
		// session yet); the token in the body is the credential.
		r.Post("/auth/local/reset/request", h.handleLocalResetRequest)
		r.Get("/auth/local/reset", h.handleLocalResetPreview)
		r.Post("/auth/local/reset", h.handleLocalResetSet)
	}
	r.Post("/auth/logout", h.handleLogout)
	// Onboarding gate (ADR-0038): authenticated by the handshake cookie, not a
	// session, so deliberately NOT behind RequireAuth.
	r.Get("/onboarding/options", h.handleOnboardingOptions)
	r.Post("/onboarding/choice", h.handleOnboardingChoice)
	r.With(RequireAuth).Get("/me", h.handleMe)
	r.With(RequireAuth).Patch("/me", h.handleUpdateMe)
	r.With(RequireAuth).Get("/household/members", h.handleListHouseholdMembers)
	r.With(RequireAuth).Patch("/household/settings", h.handleUpdateHouseholdSettings)
	r.With(RequireAuth).Post("/invitations", h.handleCreateInvitation)
}

func (h *Handlers) handleStart(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		slog.Error("generate oauth state", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookieName,
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	if invite := r.URL.Query().Get("invite"); invite != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     oauthInviteCookieName,
			Value:    invite,
			Path:     "/",
			MaxAge:   300,
			HttpOnly: true,
			Secure:   h.cookieSecure,
			SameSite: http.SameSiteLaxMode,
		})
	}

	// The pre-auth language pick rides the round-trip so a brand-new account is
	// seeded in the chosen language (ADR-0035). Only a supported BCP47 value is
	// carried; an unsupported hint is dropped and the seed falls back to en-GB.
	if lng := r.URL.Query().Get("lng"); lng != "" {
		if _, ok := supportedLocales[lng]; ok {
			http.SetCookie(w, &http.Cookie{
				Name:     oauthLocaleCookieName,
				Value:    lng,
				Path:     "/",
				MaxAge:   300,
				HttpOnly: true,
				Secure:   h.cookieSecure,
				SameSite: http.SameSiteLaxMode,
			})
		}
	}

	http.Redirect(w, r, h.googleOAuth.authCodeURL(state), http.StatusFound)
}

func (h *Handlers) handleCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stateCookie, err := r.Cookie(oauthStateCookieName)
	if err != nil || stateCookie.Value == "" {
		http.Error(w, "missing oauth_state cookie", http.StatusBadRequest)
		return
	}
	if r.URL.Query().Get("state") != stateCookie.Value {
		http.Error(w, "oauth state mismatch", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing oauth code", http.StatusBadRequest)
		return
	}

	// State has served its purpose; clear it (and the invite cookie if any —
	// we'll read its value before clearing).
	var inviteToken string
	if inviteCookie, err := r.Cookie(oauthInviteCookieName); err == nil {
		inviteToken = inviteCookie.Value
	}
	var localeHint string
	if localeCookie, err := r.Cookie(oauthLocaleCookieName); err == nil {
		localeHint = localeCookie.Value
	}
	h.clearShortCookie(w, oauthStateCookieName)
	h.clearShortCookie(w, oauthInviteCookieName)
	h.clearShortCookie(w, oauthLocaleCookieName)

	claims, err := h.googleOAuth.exchange(ctx, code)
	if err != nil {
		slog.Error("oauth exchange", "err", err)
		http.Error(w, "oauth exchange failed", http.StatusBadGateway)
		return
	}

	// inviteIgnored: an already-onboarded user arrived via a fresh invite link.
	// A User belongs to exactly one Household with no leave/switch (ADR-0017), so
	// the link can't move them — but rather than silently drop it, we carry a
	// signal to the SPA to explain why (ADR-0038). Messaging only: no membership
	// change, and the invitation is left untouched (not consumed).
	inviteIgnored := false

	user, err := h.q.GetUserByGoogleSub(ctx, claims.Sub)
	switch {
	case err == nil:
		// Existing user — refresh the Google-sourced picture if it changed.
		// This also backfills users created before picture_url existed; the
		// equality guard skips the write on steady-state logins.
		if pic := nullableString(claims.Picture); !equalStringPtr(user.PictureUrl, pic) {
			user, err = h.q.SetUserPicture(ctx, db.SetUserPictureParams{ID: user.ID, PictureUrl: pic})
			if err != nil {
				slog.Error("refresh user picture", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}
		// A non-empty token that names a real invitation triggers the notice; a
		// stale cookie or garbage token is just ignored as before.
		if inviteToken != "" {
			if _, lookupErr := h.q.GetInvitationByTokenHash(ctx, HashToken(inviteToken)); lookupErr == nil {
				inviteIgnored = true
			}
		}
	case errors.Is(err, pgx.ErrNoRows):
		// Brand-new Google identity. The founder-vs-join decision moves *after*
		// identity verification to an explicit gate (ADR-0038): record a
		// short-lived onboarding handshake and bounce to the SPA instead of
		// writing anything — no users/households row, no session yet. A clicked
		// `?invite=` link degrades from *the decision* to a pre-selection hint;
		// the gate's email-scoped lookup is authoritative.
		hint := h.resolveInviteHint(ctx, inviteToken, claims.Email)
		if err := h.beginOnboarding(ctx, w, claims, resolveSeedLocale(localeHint), hint); err != nil {
			slog.Error("begin onboarding", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, h.frontendURL+"/onboarding", http.StatusFound)
		return
	default:
		slog.Error("lookup user by google_sub", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.IssueSession(ctx, w, user.ID, r.UserAgent()); err != nil {
		slog.Error("issue session", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	dest := h.frontendURL
	if inviteIgnored {
		dest += "/?notice=invite_ignored"
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

// IssueSession mints a fresh session for userID and writes the session cookie.
// Shared by the OAuth callback and the whole-household restore: a restore wipes
// every session row, so it re-issues one for the restored caller (re-linked by
// google_sub) rather than dumping them on the sign-in screen. Must be called
// before the response status is written, since it sets a Set-Cookie header.
func (h *Handlers) IssueSession(ctx context.Context, w http.ResponseWriter, userID uuid.UUID, userAgent string) error {
	sessionID, err := randomSessionID()
	if err != nil {
		return fmt.Errorf("generate session id: %w", err)
	}
	expiresAt := time.Now().Add(h.sessionTTL)
	var ua *string
	if userAgent != "" {
		ua = &userAgent
	}
	if _, err := h.q.CreateSession(ctx, db.CreateSessionParams{
		ID:        sessionID,
		UserID:    userID,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		UserAgent: ua,
	}); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	h.setSessionCookie(w, sessionID, expiresAt)
	return nil
}

// nullableString maps an empty string to NULL (nil) and any non-empty value to
// a pointer. Used for optional Google claims (e.g. picture) where "" and absent
// are equivalent and should both store NULL.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// equalStringPtr reports whether two optional strings hold the same value,
// treating two nils as equal. Used to skip a no-op picture write on login.
func equalStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// resolveInviteHint maps a clicked `?invite=` token to the invitation it refers
// to, but *only* when that invitation is still a valid pending invite for the
// verified email — in which case its id pre-highlights that Household at the
// gate (ADR-0038). An empty, unknown, used, expired, or wrong-email token
// yields no hint (nil): the link degrades silently and the gate's email-scoped
// lookup remains the authority. Never an error path — a bad hint just isn't a
// hint.
func (h *Handlers) resolveInviteHint(ctx context.Context, inviteToken, email string) *uuid.UUID {
	if inviteToken == "" {
		return nil
	}
	invite, err := h.q.GetInvitationByTokenHash(ctx, HashToken(inviteToken))
	if err != nil {
		return nil
	}
	if invite.InvitedEmail != email || invite.UsedAt.Valid {
		return nil
	}
	if !invite.ExpiresAt.Valid || invite.ExpiresAt.Time.Before(time.Now()) {
		return nil
	}
	id := invite.ID
	return &id
}

// createFounder mints a new Household + User from verified Google claims. The
// householdName override lets the onboarding gate (ADR-0038) pass a
// user-supplied name; an empty string derives the default "{Name}'s Household".
func (h *Handlers) createFounder(ctx context.Context, c *googleClaims, seedLocale, householdName string) (db.User, error) {
	if householdName == "" {
		householdName = c.Name + "'s Household"
	}
	household, err := h.q.CreateHousehold(ctx, db.CreateHouseholdParams{
		DisplayName:       householdName,
		ReportingCurrency: "IDR",
	})
	if err != nil {
		return db.User{}, err
	}
	user, err := h.q.CreateUser(ctx, db.CreateUserParams{
		HouseholdID: household.ID,
		DisplayName: c.Name,
		Email:       c.Email,
		GoogleSub:   c.Sub,
		Locale:      seedLocale,
		TimeZone:    "Asia/Jakarta",
		PictureUrl:  nullableString(c.Picture),
		CreatedBy:   nil,
	})
	if err != nil {
		return db.User{}, err
	}

	// Best-effort welcome email (ADR-0020): a mail outage must never cost the
	// founder their signup, so we log and proceed rather than fail the flow.
	if err := h.sendWelcomeEmail(ctx, user); err != nil {
		slog.Error("send welcome email", "err", err, "user_id", user.ID)
	}
	return user, nil
}

func (h *Handlers) clearShortCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *Handlers) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		_ = h.q.DeleteSession(r.Context(), cookie.Value)
	}
	h.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

type meResponse struct {
	ID                   uuid.UUID `json:"id"`
	HouseholdID          uuid.UUID `json:"household_id"`
	HouseholdDisplayName string    `json:"household_display_name"`
	DisplayName          string    `json:"display_name"`
	Nickname             *string   `json:"nickname"`
	Email                string    `json:"email"`
	PictureURL           *string   `json:"picture_url"`
	Locale               string    `json:"locale"`
	Theme                string    `json:"theme"`
	CarryoverDateMode    string    `json:"carryover_date_mode"`
	TimeZone             string    `json:"time_zone"`
	ReportingCurrency    string    `json:"reporting_currency"`
	MultiCurrencyEnabled bool      `json:"multi_currency_enabled"`
}

func meResponseFor(user db.User, hh db.Household) meResponse {
	return meResponse{
		ID:                   user.ID,
		HouseholdID:          user.HouseholdID,
		HouseholdDisplayName: hh.DisplayName,
		DisplayName:          user.DisplayName,
		Nickname:             user.Nickname,
		Email:                user.Email,
		PictureURL:           user.PictureUrl,
		Locale:               user.Locale,
		Theme:                user.Theme,
		CarryoverDateMode:    user.CarryoverDateMode,
		TimeZone:             user.TimeZone,
		ReportingCurrency:    hh.ReportingCurrency,
		MultiCurrencyEnabled: hh.MultiCurrencyEnabled,
	}
}

func (h *Handlers) handleMe(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}
	hh, err := h.q.GetHouseholdByID(r.Context(), user.HouseholdID)
	if err != nil {
		slog.Error("get household for /me", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(meResponseFor(user, hh))
}

const maxNicknameLen = 32

// supportedLocales mirrors the allowed set in the users.locale CHECK
// (migration 00020) and the frontend's SUPPORTED_LOCALES constant. Add a new
// language by extending all three. The handler validates here so clients see
// a 400 with a friendly message rather than a 500 from the CHECK violation.
var supportedLocales = map[string]struct{}{
	"en-GB": {},
	"id-ID": {},
}

// supportedThemes mirrors the allowed set in the users.theme CHECK (migration
// 00024) and the frontend's SUPPORTED_THEMES constant. Add a new theme by
// extending all three. Validated here so a bad value is a 400 with a friendly
// message rather than a 500 from the CHECK violation.
var supportedThemes = map[string]struct{}{
	"light": {},
	"dark":  {},
}

// supportedCarryoverDateModes mirrors the allowed set in the
// users.carryover_date_mode CHECK (migration 00002) and the frontend's
// SUPPORTED_CARRYOVER_DATE_MODES constant. Add a mode by extending all three.
// Validated here so a bad value is a 400 with a friendly message rather than a
// 500 from the CHECK violation.
var supportedCarryoverDateModes = map[string]struct{}{
	"today":                            {},
	"end_of_last_month":                {},
	"end_of_month_after_last_snapshot": {},
}

// handleUpdateMe updates the current user's own profile. Editable today: the
// self-set `nickname` (the compact owner label, falling back to display_name
// on read) and the UI `locale` (ADR-0026). `display_name` stays Google-sourced
// and is not editable here. Each field is updated independently; the request
// can omit either or both. Decoding via map[string]json.RawMessage so the
// handler can distinguish "field absent" (skip) from "field present and null"
// (e.g. nickname:null clears) — a flat struct with *string fields collapses
// both states to nil.
//
// Semantics per field:
//
//	nickname: present + string → set (trimmed, blank/whitespace clears, >32 → 400)
//	          present + null   → clear
//	          absent           → unchanged
//	locale:   present + string → set (must be in supportedLocales, else 400)
//	          present + null   → 400 (no clear semantics; locale always set)
//	          absent           → unchanged
//	theme:    present + string → set (must be in supportedThemes, else 400)
//	          present + null   → 400 (no clear semantics; theme always set)
//	          absent           → unchanged
//	carryover_date_mode: present + string → set (must be in
//	          supportedCarryoverDateModes, else 400)
//	          present + null   → 400 (no clear semantics; mode always set)
//	          absent           → unchanged
func (h *Handlers) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	updated := user

	if nicknameRaw, present := raw["nickname"]; present {
		// *string unmarshals JSON null to nil; a plain string to a non-nil pointer.
		var nickname *string
		if err := json.Unmarshal(nicknameRaw, &nickname); err != nil {
			// Field-level type mismatch: surface as the same VALIDATION envelope
			// the validator would emit for a regular struct decode, so the FE
			// catalog only has to know one rendering path.
			httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
				"field": "nickname",
				"rule":  "type",
			})
			return
		}
		var stored *string
		if nickname != nil {
			if trimmed := strings.TrimSpace(*nickname); trimmed != "" {
				if utf8.RuneCountInString(trimmed) > maxNicknameLen {
					httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
						"field": "nickname",
						"rule":  "max",
					})
					return
				}
				stored = &trimmed
			}
		}
		next, err := h.q.UpdateUserNickname(r.Context(), db.UpdateUserNicknameParams{
			UpdatedBy: &user.ID,
			Nickname:  stored,
		})
		if err != nil {
			slog.Error("update user nickname", "err", err)
			httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
			return
		}
		updated = next
	}

	if localeRaw, present := raw["locale"]; present {
		var locale *string
		if err := json.Unmarshal(localeRaw, &locale); err != nil || locale == nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
				"field": "locale",
				"rule":  "required",
			})
			return
		}
		if _, ok := supportedLocales[*locale]; !ok {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
				"field": "locale",
				"rule":  "oneof",
			})
			return
		}
		next, err := h.q.UpdateUserLocale(r.Context(), db.UpdateUserLocaleParams{
			UpdatedBy: &user.ID,
			Locale:    *locale,
		})
		if err != nil {
			slog.Error("update user locale", "err", err)
			httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
			return
		}
		updated = next
	}

	if themeRaw, present := raw["theme"]; present {
		var theme *string
		if err := json.Unmarshal(themeRaw, &theme); err != nil || theme == nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
				"field": "theme",
				"rule":  "required",
			})
			return
		}
		if _, ok := supportedThemes[*theme]; !ok {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
				"field": "theme",
				"rule":  "oneof",
			})
			return
		}
		next, err := h.q.UpdateUserTheme(r.Context(), db.UpdateUserThemeParams{
			UpdatedBy: &user.ID,
			Theme:     *theme,
		})
		if err != nil {
			slog.Error("update user theme", "err", err)
			httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
			return
		}
		updated = next
	}

	if modeRaw, present := raw["carryover_date_mode"]; present {
		var mode *string
		if err := json.Unmarshal(modeRaw, &mode); err != nil || mode == nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
				"field": "carryover_date_mode",
				"rule":  "required",
			})
			return
		}
		if _, ok := supportedCarryoverDateModes[*mode]; !ok {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
				"field": "carryover_date_mode",
				"rule":  "oneof",
			})
			return
		}
		next, err := h.q.UpdateUserCarryoverDateMode(r.Context(), db.UpdateUserCarryoverDateModeParams{
			UpdatedBy:         &user.ID,
			CarryoverDateMode: *mode,
		})
		if err != nil {
			slog.Error("update user carryover date mode", "err", err)
			httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
			return
		}
		updated = next
	}

	hh, err := h.q.GetHouseholdByID(r.Context(), updated.HouseholdID)
	if err != nil {
		slog.Error("get household for update me", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(meResponseFor(updated, hh))
}

// householdMember is the public shape — only fields the frontend needs for
// the sole-owner picker. Deliberately omits google_sub, audit cols, etc.
type householdMember struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	Nickname    *string   `json:"nickname"`
	Email       string    `json:"email"`
}

func (h *Handlers) handleListHouseholdMembers(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}
	rows, err := h.q.ListUsersByHousehold(r.Context(), user.HouseholdID)
	if err != nil {
		slog.Error("list household members", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	out := make([]householdMember, 0, len(rows))
	for _, u := range rows {
		out = append(out, householdMember{
			ID:          u.ID,
			DisplayName: u.DisplayName,
			Nickname:    u.Nickname,
			Email:       u.Email,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

type householdSettings struct {
	ID                   uuid.UUID `json:"id"`
	DisplayName          string    `json:"display_name"`
	ReportingCurrency    string    `json:"reporting_currency"`
	MultiCurrencyEnabled bool      `json:"multi_currency_enabled"`
}

type updateHouseholdSettingsReq struct {
	ReportingCurrency    string `json:"reporting_currency"`
	MultiCurrencyEnabled bool   `json:"multi_currency_enabled"`
}

// handleUpdateHouseholdSettings sets the reporting currency + multi-currency
// toggle (ADR-0002). Turning multi-currency off is blocked while positions in a
// non-reporting currency still exist — their values would silently be summed as
// reporting currency.
func (h *Handlers) handleUpdateHouseholdSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}
	var req updateHouseholdSettingsReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if len(req.ReportingCurrency) != 3 {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
			"field": "reporting_currency",
			"rule":  "len",
		})
		return
	}
	if !req.MultiCurrencyEnabled {
		n, err := h.q.CountForeignCurrencyPositions(r.Context(), db.CountForeignCurrencyPositionsParams{
			HouseholdID:    user.HouseholdID,
			NativeCurrency: req.ReportingCurrency,
		})
		if err != nil {
			slog.Error("count foreign positions", "err", err)
			httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
			return
		}
		if n > 0 {
			httperr.Write(w, http.StatusConflict, httperr.CodeForeignPositionsExist, nil)
			return
		}
	}
	hh, err := h.q.UpdateHouseholdSettings(r.Context(), db.UpdateHouseholdSettingsParams{
		ID:                   user.HouseholdID,
		ReportingCurrency:    req.ReportingCurrency,
		MultiCurrencyEnabled: req.MultiCurrencyEnabled,
		UpdatedBy:            &user.ID,
	})
	if err != nil {
		slog.Error("update household settings", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(householdSettings{
		ID:                   hh.ID,
		DisplayName:          hh.DisplayName,
		ReportingCurrency:    hh.ReportingCurrency,
		MultiCurrencyEnabled: hh.MultiCurrencyEnabled,
	})
}
