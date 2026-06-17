package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/email"
)

var errInjectedSendFailure = errors.New("smtp: injected failure")

// isoDateRE matches a machine date like 2026-06-17 — the format that must NOT
// appear in human-facing email copy.
var isoDateRE = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// NotifyRestore is the third transactional sender (#176, ADR-0036): after a
// successful restore it mails the restorer a confirmation and every *other* live
// member a relocation/security notice, each in the recipient's own locale. These
// tests pin its call site — the role split, the soft-deleted exclusion, the
// per-recipient locale, and the best-effort contract (a send failure on one
// recipient never aborts the rest or surfaces an error).

// addMember creates an extra user in an existing household with a chosen locale.
func addMember(t *testing.T, q *db.Queries, householdID uuid.UUID, name, locale string) db.User {
	t.Helper()
	u, err := q.CreateUser(context.Background(), db.CreateUserParams{
		HouseholdID: householdID,
		DisplayName: name,
		Email:       name + "@example.com",
		GoogleSub:   "sub-" + name,
		Locale:      locale,
		TimeZone:    "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", name, err)
	}
	return u
}

// softDeleteUser marks a user deleted in place (no dedicated query exists).
func softDeleteUser(t *testing.T, h *authHarness, id uuid.UUID) {
	t.Helper()
	if _, err := h.pool.Exec(context.Background(),
		`UPDATE users SET deleted_at = now() WHERE id = $1`, id); err != nil {
		t.Fatalf("soft-delete user: %v", err)
	}
}

// bySubject indexes sent messages by recipient address.
func byRecipient(msgs []email.Message) map[string]email.Message {
	out := make(map[string]email.Message, len(msgs))
	for _, m := range msgs {
		out[m.To] = m
	}
	return out
}

// covers: INV-NOTIFICATIONS-08
//
// A successful restore mails exactly the live members: the restorer gets the
// "restore complete" confirmation, every other live member gets the relocation /
// security notice, and a soft-deleted member is not mailed at all. Misrouting a
// role (sending a member the restorer copy) or mailing a soft-deleted user is the
// bar this row guards.
func TestNotifyRestore_RolesAndSkipsSoftDeleted(t *testing.T) {
	h := newAuthHarness(t)
	hid := h.user.HouseholdID
	restorer := h.user // Alice, the restorer
	bob := addMember(t, h.q, hid, "Bob", "id-ID")
	gone := addMember(t, h.q, hid, "Gone", "en-GB")
	softDeleteUser(t, h, gone.ID)

	h.h.NotifyRestore(context.Background(), hid, restorer.ID, 42)

	sent := byRecipient(h.mailer.sent())
	if len(sent) != 2 {
		t.Fatalf("want 2 emails (restorer + 1 live member), got %d: %v", len(sent), h.mailer.sent())
	}
	if _, ok := sent[gone.Email]; ok {
		t.Errorf("soft-deleted member %q must not be mailed", gone.Email)
	}

	rmsg, ok := sent[restorer.Email]
	if !ok {
		t.Fatalf("restorer %q not mailed", restorer.Email)
	}
	// The restorer gets the confirmation, carrying the sanity-check item count.
	if !strings.Contains(rmsg.Subject, "dipulihkan") && !strings.Contains(rmsg.Subject, "has been restored") {
		t.Errorf("restorer subject not the confirmation: %q", rmsg.Subject)
	}
	if !strings.Contains(rmsg.HTML, "42") {
		t.Errorf("restorer email missing item count 42: %q", rmsg.HTML)
	}

	bmsg, ok := sent[bob.Email]
	if !ok {
		t.Fatalf("member %q not mailed", bob.Email)
	}
	// The member gets the security notice naming the restorer.
	if !strings.Contains(bmsg.HTML, restorer.DisplayName) {
		t.Errorf("member notice should name the restorer %q: %q", restorer.DisplayName, bmsg.HTML)
	}
	if !strings.Contains(bmsg.HTML, "amankan akun Anda") {
		t.Errorf("member notice missing the security line: %q", bmsg.HTML)
	}
}

// covers: INV-NOTIFICATIONS-09
//
// Each recipient's message renders in their *own* user.locale — not the
// restorer's — with the en-GB fallback for an unknown locale; the brand name
// "Balances" stays literal. A member reading the notice in the wrong language is
// the bar this row guards.
func TestNotifyRestore_LocaleRenderedPerRecipient(t *testing.T) {
	h := newAuthHarness(t)
	hid := h.user.HouseholdID
	// Restorer renders in en-GB; members render independently.
	if _, err := h.pool.Exec(context.Background(),
		`UPDATE users SET locale = 'en-GB' WHERE id = $1`, h.user.ID); err != nil {
		t.Fatalf("set restorer locale: %v", err)
	}
	idMember := addMember(t, h.q, hid, "Budi", "id-ID")
	enMember := addMember(t, h.q, hid, "Clara", "en-GB")

	h.h.NotifyRestore(context.Background(), hid, h.user.ID, 5)

	sent := byRecipient(h.mailer.sent())
	if got := sent[idMember.Email]; !strings.Contains(got.HTML, "dipulihkan") {
		t.Errorf("id-ID member not rendered in Indonesian: %q", got.HTML)
	}
	if got := sent[enMember.Email]; !strings.Contains(got.HTML, "was restored") {
		t.Errorf("en-GB member not rendered in English: %q", got.HTML)
	}
	// The notice date is human-readable + localized, never a machine ISO date.
	idMonth := monthNames["id-ID"][int(time.Now().Month())-1]
	if got := sent[idMember.Email]; !strings.Contains(got.HTML, idMonth) {
		t.Errorf("id-ID notice should carry a localized month name %q: %q", idMonth, got.HTML)
	}
	if got := sent[idMember.Email]; isoDateRE.MatchString(got.HTML) {
		t.Errorf("id-ID notice leaked a machine ISO date: %q", got.HTML)
	}
	// Brand stays literal in every locale.
	if got := sent[idMember.Email]; !strings.Contains(got.HTML, "Balances") {
		t.Errorf("brand name should stay literal even in id-ID: %q", got.HTML)
	}
}

// covers: INV-NOTIFICATIONS-09
//
// localizedDate renders a human-readable, locale- and timezone-aware date — never
// a machine ISO string. The same instant near a day boundary lands on different
// calendar days in different zones, and month names translate per locale.
func TestLocalizedDate(t *testing.T) {
	// 2026-06-17 23:30 UTC is already 2026-06-18 06:30 in Jakarta (UTC+7).
	instant := time.Date(2026, time.June, 17, 23, 30, 0, 0, time.UTC)

	if got := localizedDate(instant, "en-GB", "UTC"); got != "17 June 2026" {
		t.Errorf("en-GB/UTC = %q, want %q", got, "17 June 2026")
	}
	if got := localizedDate(instant, "id-ID", "Asia/Jakarta"); got != "18 Juni 2026" {
		t.Errorf("id-ID/Jakarta = %q, want %q", got, "18 Juni 2026")
	}
	// Unknown locale falls back to en-GB month names; bad tz falls back to UTC.
	if got := localizedDate(instant, "fr-FR", "Not/AZone"); got != "17 June 2026" {
		t.Errorf("fallback = %q, want %q", got, "17 June 2026")
	}
}

// countingFailMailer fails every Send but records each attempt, so a test can
// prove the best-effort loop attempted all recipients despite failures.
type countingFailMailer struct {
	mu       sync.Mutex
	attempts []string
}

func (m *countingFailMailer) Send(_ context.Context, msg email.Message) error {
	m.mu.Lock()
	m.attempts = append(m.attempts, msg.To)
	m.mu.Unlock()
	return errInjectedSendFailure
}

// covers: INV-NOTIFICATIONS-02
//
// Best-effort, non-blocking (ADR-0020) generalized to the restore sender: a
// mailer.Send failure on one recipient is swallowed and never aborts the loop or
// surfaces — NotifyRestore returns nothing for the caller to fail on, and every
// live member is still attempted. A mail outage must never reflect on the
// restore that already committed.
func TestNotifyRestore_BestEffortAttemptsAll(t *testing.T) {
	h := newAuthHarness(t)
	hid := h.user.HouseholdID
	fm := &countingFailMailer{}
	h.h.mailer = fm
	addMember(t, h.q, hid, "Bob", "id-ID")
	addMember(t, h.q, hid, "Cara", "en-GB")

	// Must not panic or block despite every Send failing.
	h.h.NotifyRestore(context.Background(), hid, h.user.ID, 1)

	// All three live members were attempted; the first failure did not abort.
	if len(fm.attempts) != 3 {
		t.Errorf("best-effort should attempt all 3 live members despite failures, got %d: %v", len(fm.attempts), fm.attempts)
	}
}
