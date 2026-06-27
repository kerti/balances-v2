package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// Local invite accept (ADR-0039/#281). An invited household member with no Google
// account joins by following their invite link and setting a password. Possession
// of the single-use link IS the email proof — the local mirror of Google's
// email-match check — so the account binds to invited_email with NO second
// ADR-0038 verified-email gate. There is one decision path here, not a
// founder-vs-join choice: the invitation already names the Household.

type localInvitePreviewResp struct {
	// InvitedEmail is the address the new account binds to. The link holder
	// already proved possession, so showing the bound email is a help, not a leak.
	InvitedEmail string `json:"invited_email"`
	// HouseholdName lets the screen say which household the invitee is joining.
	HouseholdName string `json:"household_name"`
}

// handleLocalInvitePreview resolves a set-password link for the accept screen
// WITHOUT consuming it: it returns the bound email + household name when the
// hashed token names a still-pending, unexpired invitation, else the same generic
// 409 the consume path uses. A read-only GET, so a user reloading the form does
// not burn their single-use link — consumption happens only at POST accept.
func (h *Handlers) handleLocalInvitePreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := r.URL.Query().Get("token")
	if token == "" {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
			"field": "token", "rule": "required",
		})
		return
	}

	invite, err := h.q.GetInvitationByTokenHash(ctx, HashToken(token))
	if err != nil || !inviteAcceptable(invite) {
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			slog.Error("invite preview: lookup", "err", err)
			httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
			return
		}
		// Unknown / used / expired link — one generic answer (the SPA shows a
		// "this link is no longer valid" message).
		httperr.Write(w, http.StatusConflict, httperr.CodeInvitationNoLongerValid, nil)
		return
	}

	household, err := h.q.GetHouseholdByID(ctx, invite.HouseholdID)
	if err != nil {
		slog.Error("invite preview: lookup household", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(localInvitePreviewResp{
		InvitedEmail:  invite.InvitedEmail,
		HouseholdName: household.DisplayName,
	})
}

type localInviteAcceptReq struct {
	Token    string `json:"token"`
	Password string `json:"password"`
	// Locale is the inviter's language carried on the link (?lng=), echoed back by
	// the SPA so the new member inherits the household language (ADR-0035). Falls
	// back to the default when absent/unsupported.
	Locale string `json:"locale"`
}

// handleLocalInviteAccept creates a local account from a set-password link and
// mints a session. The invitation is consumed and the User+credential created in
// ONE transaction (ADR-0039/#281): the conditional consume (used_at IS NULL AND
// not expired) is the single-use guard — under concurrent accepts the second
// UPDATE re-evaluates after the first commits and matches zero rows, so a
// consumed/expired/forwarded-after-use link can never mint a second account. A
// mid-transaction failure rolls the consume back, so the link is never burned
// without an account to show for it.
func (h *Handlers) handleLocalInviteAccept(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req localInviteAcceptReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if req.Token == "" {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
			"field": "token", "rule": "required",
		})
		return
	}
	if err := ValidatePasswordPolicy(req.Password); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeWeakPassword, map[string]any{
			"reason": weakPasswordReason(err),
		})
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		slog.Error("invite accept: hash password", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	user, err := h.commitLocalInvite(ctx, HashToken(req.Token), hash, resolveSeedLocale(req.Locale))
	switch {
	case errors.Is(err, errInviteNotAcceptable):
		httperr.Write(w, http.StatusConflict, httperr.CodeInvitationNoLongerValid, nil)
		return
	case errors.Is(err, errInviteEmailTaken):
		// The invited address already owns an account (joined another way). The
		// link holder possesses a token bound to that email, so naming the clash
		// is not enumeration.
		httperr.Write(w, http.StatusConflict, httperr.CodeEmailTaken, nil)
		return
	case err != nil:
		slog.Error("invite accept: commit", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	// Session is issued only after the account commits — no ADR-0038 gate, the
	// invitee lands straight in the app.
	if err := h.IssueSession(ctx, w, user.ID, r.UserAgent()); err != nil {
		slog.Error("invite accept: issue session", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var (
	errInviteNotAcceptable = errors.New("invitation not acceptable")
	errInviteEmailTaken    = errors.New("invited email already has an account")
)

// commitLocalInvite runs the accept as a single transaction: atomically consume
// the invitation, then create the google_sub-less User bound to invited_email and
// its local_credentials row from the already-hashed password. Returns
// errInviteNotAcceptable when the conditional consume matches nothing, and
// errInviteEmailTaken on the email-uniqueness clash; both roll the consume back.
func (h *Handlers) commitLocalInvite(ctx context.Context, tokenHash, passwordHash, seedLocale string) (db.User, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return db.User{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful Commit

	qtx := h.q.WithTx(tx)

	invite, err := qtx.ConsumeInvitationByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.User{}, errInviteNotAcceptable
		}
		return db.User{}, err
	}

	user, err := qtx.CreateLocalUser(ctx, db.CreateLocalUserParams{
		HouseholdID: invite.HouseholdID,
		DisplayName: emailLocalPart(invite.InvitedEmail),
		Email:       invite.InvitedEmail,
		Locale:      seedLocale,
		TimeZone:    "Asia/Jakarta",
		PictureUrl:  nil,
		CreatedBy:   &invite.CreatedBy,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.User{}, errInviteEmailTaken
		}
		return db.User{}, err
	}

	if _, err := qtx.UpsertLocalCredential(ctx, db.UpsertLocalCredentialParams{
		UserID:       user.ID,
		PasswordHash: passwordHash,
	}); err != nil {
		return db.User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return db.User{}, err
	}
	return user, nil
}

// inviteAcceptable reports whether an invitation row is still pending and within
// its TTL — the read-only check the preview uses. The POST accept does not rely on
// it: there the conditional UPDATE re-checks both facts atomically.
func inviteAcceptable(inv db.HouseholdInvitation) bool {
	if inv.UsedAt.Valid {
		return false
	}
	return inv.ExpiresAt.Valid && inv.ExpiresAt.Time.After(time.Now())
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
