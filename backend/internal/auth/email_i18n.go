package auth

import (
	"fmt"
	"time"
)

// Per-locale transactional email copy (ADR-0035). A hand-rolled catalog keyed by
// BCP47 — the emails render in Go through email.Layout, outside react-i18next's
// reach, and the scale (a handful of plural-free emails × 2 locales) doesn't
// warrant a message-file library. Subjects are localized too; the product name
// "Balances" stays literal in every locale (it is the brand, not a translatable
// string). A recipient locale with no catalog entry falls back to en-GB.

// welcomeCopy is the founder welcome email's translatable strings. greeting
// carries one %s (the founder display name); the rest are static paragraphs.
type welcomeCopy struct {
	subject  string
	greeting string // %s = founder display name
	intro    string
	invite   string
	cta      string
	signoff  string // plain-text part only
}

var welcomeCatalog = map[string]welcomeCopy{
	"en-GB": {
		subject:  "Welcome to Balances",
		greeting: "Welcome, %s!",
		intro:    "Balances helps your household see its net worth in one place — bank accounts, property, investments, and what you owe. Each month you enter your balances from your statements, and Balances tracks how your total moves over time.",
		invite:   "It works best with everyone in. Invite the people you share finances with so you're all looking at the same picture.",
		cta:      "Invite your household",
		signoff:  "— the Balances team",
	},
	"id-ID": {
		subject:  "Selamat datang di Balances",
		greeting: "Selamat datang, %s!",
		intro:    "Balances membantu rumah tangga Anda melihat kekayaan bersihnya dalam satu tempat — rekening bank, properti, investasi, dan utang Anda. Setiap bulan Anda memasukkan saldo dari laporan rekening, dan Balances melacak pergerakan total Anda dari waktu ke waktu.",
		invite:   "Aplikasi ini bekerja paling baik bila semua orang ikut serta. Undang orang-orang yang berbagi keuangan dengan Anda agar kalian melihat gambaran yang sama.",
		cta:      "Undang rumah tangga Anda",
		signoff:  "— tim Balances",
	},
}

// invitationCopy is the household-invitation email's translatable strings.
type invitationCopy struct {
	subject  string // %s = inviter display name
	greeting string
	body     string // 1st %s = inviter display name, 2nd %s = household name
	linkText string
	expiry   string // %s = formatted expiry timestamp
}

var invitationCatalog = map[string]invitationCopy{
	"en-GB": {
		subject:  "%s invited you to Balances",
		greeting: "Hi,",
		body:     "%s has invited you to join their Balances household %s.",
		linkText: "Click here to accept the invitation",
		expiry:   "The link expires on %s. If you weren't expecting this email, you can safely ignore it.",
	},
	"id-ID": {
		subject:  "%s mengundang Anda ke Balances",
		greeting: "Halo,",
		body:     "%s mengundang Anda untuk bergabung dengan rumah tangga Balances mereka %s.",
		linkText: "Klik di sini untuk menerima undangan",
		expiry:   "Tautan ini kedaluwarsa pada %s. Jika Anda tidak mengharapkan email ini, Anda dapat mengabaikannya dengan aman.",
	},
}

// restoreConfirmCopy is the restorer's "restore complete" confirmation email
// (#176, ADR-0036). greeting carries one %s (the restorer display name); intro
// carries one %d (the total item count) as a sanity check.
type restoreConfirmCopy struct {
	subject  string
	greeting string // %s = restorer display name
	intro    string // %d = total restored item count
	cta      string
	signoff  string // plain-text part only
}

var restoreConfirmCatalog = map[string]restoreConfirmCopy{
	"en-GB": {
		subject:  "Your Balances household has been restored",
		greeting: "Hi, %s!",
		intro:    "Your household has been restored from a backup — %d records are now in place. Open Balances at the address below to check everything looks right.",
		cta:      "Open Balances",
		signoff:  "— the Balances team",
	},
	"id-ID": {
		subject:  "Rumah tangga Balances Anda telah dipulihkan",
		greeting: "Halo, %s!",
		intro:    "Rumah tangga Anda telah dipulihkan dari cadangan — %d catatan kini tersedia. Buka Balances di alamat di bawah untuk memastikan semuanya sudah benar.",
		cta:      "Buka Balances",
		signoff:  "— tim Balances",
	},
}

// restoreNoticeCopy is the relocation + security notice sent to every *other*
// live member after a restore (#176, ADR-0036). It doubles as a tamper
// tripwire: an unexpected notice means someone restored the household. body
// carries the restorer name (%s) and the restore date (%s); security is the
// "if this wasn't you" line.
type restoreNoticeCopy struct {
	subject  string
	greeting string // %s = member display name
	body     string // 1st %s = restorer display name, 2nd %s = restore date
	action   string
	cta      string
	security string
	signoff  string // plain-text part only
}

var restoreNoticeCatalog = map[string]restoreNoticeCopy{
	"en-GB": {
		subject:  "Your Balances household was restored",
		greeting: "Hi, %s!",
		body:     "Your Balances household was restored from a backup by %s on %s.",
		action:   "Sign in at the address below to continue where you left off.",
		cta:      "Sign in to Balances",
		security: "If you weren't expecting this, secure your account — sign in and review your household straight away.",
		signoff:  "— the Balances team",
	},
	"id-ID": {
		subject:  "Rumah tangga Balances Anda telah dipulihkan",
		greeting: "Halo, %s!",
		body:     "Rumah tangga Balances Anda dipulihkan dari cadangan oleh %s pada %s.",
		action:   "Masuk di alamat di bawah untuk melanjutkan dari tempat Anda terakhir berhenti.",
		cta:      "Masuk ke Balances",
		security: "Jika Anda tidak mengharapkan hal ini, amankan akun Anda — masuk dan periksa rumah tangga Anda segera.",
		signoff:  "— tim Balances",
	},
}

// localizedEmail returns the catalog entry for a recipient locale, falling back
// to the en-GB default when the locale has no entry (ADR-0035).
func localizedEmail[T any](catalog map[string]T, locale string) T {
	if c, ok := catalog[locale]; ok {
		return c
	}
	return catalog[defaultSeedLocale]
}

// monthNames holds the full month names per supported locale, indexed by
// time.Month-1. Go's stdlib date formatting is English-only and x/text exposes
// no public localized date formatter (its CLDR month data is in an internal/
// package), so a localized human-readable date (e.g. the restore notice's "on
// <date>") needs its own table. Hand-rolled for the same reason ADR-0035 chose a
// hand-rolled copy catalog: at two locales a library is pure surface, since the
// maintained options (monday, lctime) are themselves just tables like this one.
// Both supported locales render day-month-year.
//
// WHEN ADDING A LOCALE: add a row here too, not just to the copy catalogs above —
// a missing locale silently falls back to en-GB month names (localizedDate),
// which would read wrong rather than fail loudly. The supported set is also
// pinned by the DB users_locale_check constraint and the frontend i18n bundles.
var monthNames = map[string][]string{
	"en-GB": {"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December"},
	"id-ID": {"Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember"},
}

// localizedDate renders t as a human-readable "D Month YYYY" in the recipient's
// locale and time zone (a machine-format 2026-06-17 is not for people). An
// unknown locale falls back to en-GB month names; an unparseable/empty time zone
// falls back to UTC. Both supported locales use day-month-year order.
func localizedDate(t time.Time, locale, tz string) string {
	if loc, err := time.LoadLocation(tz); err == nil {
		t = t.In(loc)
	}
	names, ok := monthNames[locale]
	if !ok {
		names = monthNames[defaultSeedLocale]
	}
	return fmt.Sprintf("%d %s %d", t.Day(), names[int(t.Month())-1], t.Year())
}
