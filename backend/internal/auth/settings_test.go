package auth

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/db"
)

func TestHandleUpdateHouseholdSettings(t *testing.T) {
	h := newAuthHarness(t)

	t.Run("200 enable multi-currency", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/household/settings", map[string]any{
			"display_name": "Alice's Household", "reporting_currency": "IDR", "multi_currency_enabled": true,
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[householdSettings](t, rec)
		if !body.MultiCurrencyEnabled {
			t.Errorf("multi_currency_enabled: got false, want true")
		}
	})

	t.Run("400 bad reporting currency", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/household/settings", map[string]any{
			"display_name": "Alice's Household", "reporting_currency": "RUPIAH", "multi_currency_enabled": true,
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("409 disable while foreign positions exist", func(t *testing.T) {
		if _, err := h.q.CreateAsset(context.Background(), db.CreateAssetParams{
			HouseholdID: h.user.HouseholdID, DisplayName: "USD acct", Subtype: "bank_account",
			OwnershipType: "joint", NativeCurrency: "USD", CreatedBy: &h.user.ID,
		}); err != nil {
			t.Fatalf("seed USD asset: %v", err)
		}
		rec := h.do(t, "PATCH", "/household/settings", map[string]any{
			"display_name": "Alice's Household", "reporting_currency": "IDR", "multi_currency_enabled": false,
		})
		requireStatus(t, rec, http.StatusConflict)
	})

	t.Run("401 unauthenticated", func(t *testing.T) {
		rec := h.doRaw(t, "PATCH", "/household/settings", map[string]any{
			"display_name": "Alice's Household", "reporting_currency": "IDR", "multi_currency_enabled": true,
		}, nil)
		requireStatus(t, rec, http.StatusUnauthorized)
	})

	t.Run("200 renames the household", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/household/settings", map[string]any{
			"display_name": "The Newlyweds", "reporting_currency": "IDR", "multi_currency_enabled": true,
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[householdSettings](t, rec)
		if body.DisplayName != "The Newlyweds" {
			t.Fatalf("display_name: got %q, want \"The Newlyweds\"", body.DisplayName)
		}
		meRec := h.do(t, "GET", "/me", nil)
		requireStatus(t, meRec, http.StatusOK)
		if got := decodeBody[meResponse](t, meRec).HouseholdDisplayName; got != "The Newlyweds" {
			t.Errorf("persisted household_display_name: got %q, want \"The Newlyweds\"", got)
		}
	})

	t.Run("200 trims surrounding whitespace", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/household/settings", map[string]any{
			"display_name": "  Spaced Out  ", "reporting_currency": "IDR", "multi_currency_enabled": true,
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[householdSettings](t, rec)
		if body.DisplayName != "Spaced Out" {
			t.Fatalf("display_name: got %q, want \"Spaced Out\"", body.DisplayName)
		}
	})

	t.Run("400 empty display name", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/household/settings", map[string]any{
			"display_name": "   ", "reporting_currency": "IDR", "multi_currency_enabled": false,
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 display name over 60 chars", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/household/settings", map[string]any{
			"display_name": strings.Repeat("a", 61), "reporting_currency": "IDR", "multi_currency_enabled": false,
		})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("200 exactly 60 chars", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/household/settings", map[string]any{
			"display_name": strings.Repeat("a", 60), "reporting_currency": "IDR", "multi_currency_enabled": true,
		})
		requireStatus(t, rec, http.StatusOK)
	})
}

func TestHandleMe_IncludesCurrencySettings(t *testing.T) {
	h := newAuthHarness(t)
	rec := h.do(t, "GET", "/me", nil)
	requireStatus(t, rec, http.StatusOK)
	body := decodeBody[meResponse](t, rec)
	if body.ReportingCurrency != "IDR" {
		t.Errorf("reporting_currency: got %q, want IDR", body.ReportingCurrency)
	}
	if body.Nickname != nil {
		t.Errorf("nickname: got %q, want nil (unset by default)", *body.Nickname)
	}
	if body.PictureURL != nil {
		t.Errorf("picture_url: got %q, want nil (unset by default)", *body.PictureURL)
	}
	if body.CarryoverDateMode != "today" {
		t.Errorf("carryover_date_mode: got %q, want \"today\" (default)", body.CarryoverDateMode)
	}
}

func TestMeResponseFor_MapsPicture(t *testing.T) {
	pic := "https://lh3.googleusercontent.com/a/pic.jpg"
	got := meResponseFor(db.User{PictureUrl: &pic}, db.Household{})
	if got.PictureURL == nil || *got.PictureURL != pic {
		t.Errorf("PictureURL: want %q, got %v", pic, got.PictureURL)
	}
}

// aliceNickname reads Alice's nickname from the DB-backed members list (the
// context user the harness injects is a stale snapshot, so /me wouldn't reflect
// a fresh PATCH — the members handler re-queries).
func aliceNickname(t *testing.T, h *authHarness) *string {
	t.Helper()
	rec := h.do(t, "GET", "/household/members", nil)
	requireStatus(t, rec, http.StatusOK)
	for _, m := range decodeBody[[]householdMember](t, rec) {
		if m.ID == h.user.ID {
			return m.Nickname
		}
	}
	t.Fatalf("alice not found in household members")
	return nil
}

func TestHandleUpdateMe(t *testing.T) {
	h := newAuthHarness(t)

	t.Run("200 sets nickname", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{"nickname": "Al"})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[meResponse](t, rec)
		if body.Nickname == nil || *body.Nickname != "Al" {
			t.Fatalf("nickname: got %v, want \"Al\"", body.Nickname)
		}
		if got := aliceNickname(t, h); got == nil || *got != "Al" {
			t.Errorf("persisted nickname: got %v, want \"Al\"", got)
		}
	})

	t.Run("200 trims surrounding whitespace", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{"nickname": "  Ally  "})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[meResponse](t, rec)
		if body.Nickname == nil || *body.Nickname != "Ally" {
			t.Fatalf("nickname: got %v, want \"Ally\"", body.Nickname)
		}
	})

	t.Run("200 empty string clears to null", func(t *testing.T) {
		// seed a value first so we observe the clear
		_ = h.do(t, "PATCH", "/me", map[string]any{"nickname": "Al"})
		rec := h.do(t, "PATCH", "/me", map[string]any{"nickname": ""})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[meResponse](t, rec)
		if body.Nickname != nil {
			t.Fatalf("nickname: got %q, want nil", *body.Nickname)
		}
		if got := aliceNickname(t, h); got != nil {
			t.Errorf("persisted nickname: got %q, want nil", *got)
		}
	})

	t.Run("200 whitespace-only clears to null", func(t *testing.T) {
		_ = h.do(t, "PATCH", "/me", map[string]any{"nickname": "Al"})
		rec := h.do(t, "PATCH", "/me", map[string]any{"nickname": "   "})
		requireStatus(t, rec, http.StatusOK)
		if body := decodeBody[meResponse](t, rec); body.Nickname != nil {
			t.Fatalf("nickname: got %q, want nil", *body.Nickname)
		}
	})

	t.Run("400 over 32 chars", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{"nickname": strings.Repeat("a", 33)})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("200 exactly 32 chars", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{"nickname": strings.Repeat("a", 32)})
		requireStatus(t, rec, http.StatusOK)
	})

	t.Run("400 malformed json", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", "{not json")
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("401 unauthenticated", func(t *testing.T) {
		rec := h.doRaw(t, "PATCH", "/me", map[string]any{"nickname": "Al"}, nil)
		requireStatus(t, rec, http.StatusUnauthorized)
	})

	t.Run("200 sets locale to en-GB", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{"locale": "en-GB"})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[meResponse](t, rec)
		if body.Locale != "en-GB" {
			t.Fatalf("locale: got %q, want en-GB", body.Locale)
		}
	})

	t.Run("200 sets locale to id-ID", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{"locale": "id-ID"})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[meResponse](t, rec)
		if body.Locale != "id-ID" {
			t.Fatalf("locale: got %q, want id-ID", body.Locale)
		}
	})

	t.Run("400 unsupported locale", func(t *testing.T) {
		// fr-FR is well-formed BCP47 but not in supportedLocales.
		rec := h.do(t, "PATCH", "/me", map[string]any{"locale": "fr-FR"})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 garbage locale", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{"locale": "not-a-locale"})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("200 sets theme to light", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{"theme": "light"})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[meResponse](t, rec)
		if body.Theme != "light" {
			t.Fatalf("theme: got %q, want light", body.Theme)
		}
	})

	t.Run("200 sets theme to dark", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{"theme": "dark"})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[meResponse](t, rec)
		if body.Theme != "dark" {
			t.Fatalf("theme: got %q, want dark", body.Theme)
		}
	})

	t.Run("400 unsupported theme", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{"theme": "sepia"})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	for _, mode := range []string{"today", "end_of_last_month", "end_of_month_after_last_snapshot"} {
		t.Run("200 sets carryover_date_mode to "+mode, func(t *testing.T) {
			rec := h.do(t, "PATCH", "/me", map[string]any{"carryover_date_mode": mode})
			requireStatus(t, rec, http.StatusOK)
			body := decodeBody[meResponse](t, rec)
			if body.CarryoverDateMode != mode {
				t.Fatalf("carryover_date_mode: got %q, want %q", body.CarryoverDateMode, mode)
			}
		})
	}

	t.Run("400 unsupported carryover_date_mode", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{"carryover_date_mode": "yesterday"})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("400 null carryover_date_mode", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{"carryover_date_mode": nil})
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("200 nickname + locale + theme + carryover together", func(t *testing.T) {
		rec := h.do(t, "PATCH", "/me", map[string]any{
			"nickname":            "Bee",
			"locale":              "en-GB",
			"theme":               "light",
			"carryover_date_mode": "end_of_last_month",
		})
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[meResponse](t, rec)
		if body.Locale != "en-GB" {
			t.Errorf("locale: got %q, want en-GB", body.Locale)
		}
		if body.Theme != "light" {
			t.Errorf("theme: got %q, want light", body.Theme)
		}
		if body.CarryoverDateMode != "end_of_last_month" {
			t.Errorf("carryover_date_mode: got %q, want end_of_last_month", body.CarryoverDateMode)
		}
		if body.Nickname == nil || *body.Nickname != "Bee" {
			t.Errorf("nickname: got %v, want \"Bee\"", body.Nickname)
		}
	})
}
