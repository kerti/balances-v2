package auth

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

// localizedEmail returns the catalog entry for a recipient locale, falling back
// to the en-GB default when the locale has no entry (ADR-0035).
func localizedEmail[T any](catalog map[string]T, locale string) T {
	if c, ok := catalog[locale]; ok {
		return c
	}
	return catalog[defaultSeedLocale]
}
