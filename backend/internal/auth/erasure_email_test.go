package auth

import (
	"context"
	"strings"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// NotifyErasure is the fourth transactional sender (#300, ADR-0040): after a
// household erasure it mails the founder a "deleted" confirmation and every
// *other* member a deletion notice, each in the recipient's own locale. Unlike
// NotifyRestore, the member list is captured by the caller before the wipe —
// these tests pass it in directly rather than seeding a live household.

// covers: INV-NOTIFICATIONS-12
//
// A successful erasure mails exactly the members it was handed: the founder
// gets the "deleted" confirmation, every other member gets the deletion
// notice — misrouting a role is the bar this row guards.
func TestNotifyErasure_Roles(t *testing.T) {
	h := newAuthHarness(t)
	founder := h.user
	peer := addMember(t, h.q, founder.HouseholdID, "Peer", "en-GB")

	h.h.NotifyErasure(context.Background(), []db.User{founder, peer}, founder.ID, "The Test Household")

	sent := byRecipient(h.mailer.sent())
	if len(sent) != 2 {
		t.Fatalf("want 2 emails (founder + peer), got %d: %v", len(sent), h.mailer.sent())
	}

	fmsg, ok := sent[founder.Email]
	if !ok {
		t.Fatalf("founder %q not mailed", founder.Email)
	}
	if !strings.Contains(fmsg.HTML, "The Test Household") {
		t.Errorf("founder confirmation missing household name: %q", fmsg.HTML)
	}
	// CreateHouseholdWithUser seeds id-ID by default (testutil/fixtures.go), so
	// the founder's own confirmation renders in Indonesian here.
	if !strings.Contains(fmsg.HTML, "secara permanen") {
		t.Errorf("founder message should be the confirmation: %q", fmsg.HTML)
	}

	pmsg, ok := sent[peer.Email]
	if !ok {
		t.Fatalf("peer %q not mailed", peer.Email)
	}
	if !strings.Contains(pmsg.HTML, "no longer exist") {
		t.Errorf("peer message should be the deletion notice: %q", pmsg.HTML)
	}
}

// covers: INV-NOTIFICATIONS-13
//
// Each recipient's erasure email renders in their own user.locale, with the
// en-GB fallback and literal brand name — the per-recipient re-pin of
// INV-NOTIFICATIONS-06/07 for the erasure sender.
func TestNotifyErasure_LocaleRenderedPerRecipient(t *testing.T) {
	h := newAuthHarness(t)
	founder := h.user
	idMember := addMember(t, h.q, founder.HouseholdID, "Budi", "id-ID")

	h.h.NotifyErasure(context.Background(), []db.User{founder, idMember}, founder.ID, "Rumah Tangga Uji")

	sent := byRecipient(h.mailer.sent())
	got, ok := sent[idMember.Email]
	if !ok {
		t.Fatalf("id-ID member %q not mailed", idMember.Email)
	}
	if !strings.Contains(got.HTML, "dihapus") {
		t.Errorf("id-ID member not rendered in Indonesian: %q", got.HTML)
	}
	if !strings.Contains(got.HTML, "Balances") {
		t.Errorf("brand name should stay literal even in id-ID: %q", got.HTML)
	}
}

// covers: INV-NOTIFICATIONS-02
//
// Best-effort, non-blocking (ADR-0020) generalized to the erasure sender: a
// mailer.Send failure on one recipient is swallowed and never aborts the loop
// — every member handed in is still attempted.
func TestNotifyErasure_BestEffortAttemptsAll(t *testing.T) {
	h := newAuthHarness(t)
	founder := h.user
	peer := addMember(t, h.q, founder.HouseholdID, "Peer", "en-GB")
	fm := &countingFailMailer{}
	h.h.mailer = fm

	h.h.NotifyErasure(context.Background(), []db.User{founder, peer}, founder.ID, "Household")

	if len(fm.attempts) != 2 {
		t.Errorf("best-effort should attempt both members despite failures, got %d: %v", len(fm.attempts), fm.attempts)
	}
}
