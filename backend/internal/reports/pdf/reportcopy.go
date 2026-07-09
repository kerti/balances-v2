package pdf

import "fmt"

// Localized copy for the PDF report (ADR-0045), a hand-rolled per-locale catalog
// keyed by BCP-47 — the same pattern as auth/email_i18n.go (the report renders
// in Go, outside react-i18next's reach, and the scale doesn't warrant a
// message-file library). A locale with no catalog entry falls back to en-GB.
// The product name "Balances" stays literal in every locale.

type reportCopy struct {
	title    string // report title
	subtitle string // %s = month + year
	netWorth string
	baseline string // shown when there's no prior month (no income statement)

	// section headings
	statistics  string
	assets      string
	liabilities string
	investments string
	receivables string
	cashFlow    string
	charts      string

	currentAssets    string
	nonCurrentAssets string

	// cash-flow rows
	cashIn      string
	cashOut     string
	income      string
	expenses    string
	netCashFlow string
	joint       string

	totalPrefix string // "Total" / "Jumlah" — combined with a section name

	fxTitle string
	fxLine  string // 1 %[1]s = %[2]s %[3]s

	staleNote string // footnote for carried-forward values

	statsSoon string // #412 placeholder caption

	// chart titles
	chartAssets      string
	chartInvestments string
	chartLiabilities string
	chartTrend       string
}

var reportCatalog = map[string]reportCopy{
	"en-GB": {
		title:            "Monthly Financial Report",
		subtitle:         "Figures reflect the household's financial position on the last day of %s.",
		netWorth:         "Net Worth",
		baseline:         "First reported month — no prior month to compare against.",
		statistics:       "Statistics",
		assets:           "Assets",
		liabilities:      "Liabilities",
		investments:      "Investments",
		receivables:      "Receivables",
		cashFlow:         "Cash Flow",
		charts:           "Charts",
		currentAssets:    "Current Assets",
		nonCurrentAssets: "Non-current Assets",
		cashIn:           "Cash In",
		cashOut:          "Cash Out",
		income:           "Income",
		expenses:         "Living Expenses",
		netCashFlow:      "Net Cash Flow",
		joint:            "Joint",
		totalPrefix:      "Total",
		fxTitle:          "Exchange Rates Used",
		fxLine:           "1 %[1]s = %[2]s %[3]s",
		staleNote:        "* carried forward from an earlier month's statement",
		statsSoon:        "Financial-health indicators are coming soon.",
		chartAssets:      "Assets Composition",
		chartInvestments: "Investments Composition",
		chartLiabilities: "Liabilities Composition",
		chartTrend:       "Net Worth — last 12 months",
	},
	"id-ID": {
		title:            "Laporan Keuangan Bulanan",
		subtitle:         "Data menunjukkan posisi keuangan rumah tangga pada hari terakhir bulan %s.",
		netWorth:         "Kekayaan Bersih",
		baseline:         "Bulan pertama yang dilaporkan — belum ada bulan sebelumnya untuk dibandingkan.",
		statistics:       "Statistika",
		assets:           "Harta",
		liabilities:      "Hutang",
		investments:      "Investasi",
		receivables:      "Piutang",
		cashFlow:         "Arus Kas",
		charts:           "Grafik",
		currentAssets:    "Harta Lancar",
		nonCurrentAssets: "Harta Tidak Lancar",
		cashIn:           "Kas Masuk",
		cashOut:          "Kas Keluar",
		income:           "Pendapatan",
		expenses:         "Pengeluaran",
		netCashFlow:      "Arus Kas Bersih",
		joint:            "Bersama",
		totalPrefix:      "Jumlah",
		fxTitle:          "Kurs yang Digunakan",
		fxLine:           "1 %[1]s = %[2]s %[3]s",
		staleNote:        "* nilai dibawa dari laporan bulan sebelumnya",
		statsSoon:        "Indikator kesehatan keuangan akan segera hadir.",
		chartAssets:      "Komposisi Harta",
		chartInvestments: "Komposisi Investasi",
		chartLiabilities: "Komposisi Hutang",
		chartTrend:       "Kekayaan Bersih — 12 Bulan Terakhir",
	},
}

// subtypeLabels maps a domain subtype to its localized display name.
var subtypeLabels = map[string]map[string]string{
	"en-GB": {
		"bank_account":  "Bank Accounts",
		"property":      "Property",
		"vehicle":       "Vehicles",
		"institutional": "Institutional Debt",
		"personal":      "Personal Debt",
		"stock":         "Stocks",
		"mutual_fund":   "Mutual Funds",
		"bond":          "Bonds",
		"gold":          "Gold",
		"time_deposit":  "Time Deposits",
	},
	"id-ID": {
		"bank_account":  "Rekening Bank",
		"property":      "Properti",
		"vehicle":       "Kendaraan",
		"institutional": "Hutang Lembaga",
		"personal":      "Hutang Pribadi",
		"stock":         "Saham",
		"mutual_fund":   "Reksa Dana",
		"bond":          "Obligasi",
		"gold":          "Emas",
		"time_deposit":  "Deposito",
	},
}

const fallbackLocale = "en-GB"

// JointLabel is the localized "Joint" owner label, exported so the input
// builder can resolve joint-owned positions without duplicating the catalog.
func JointLabel(locale string) string { return copyFor(locale).joint }

func copyFor(locale string) reportCopy {
	if c, ok := reportCatalog[locale]; ok {
		return c
	}
	return reportCatalog[fallbackLocale]
}

func subtypeLabel(locale, subtype string) string {
	if m, ok := subtypeLabels[locale]; ok {
		if s, ok := m[subtype]; ok {
			return s
		}
	}
	if s, ok := subtypeLabels[fallbackLocale][subtype]; ok {
		return s
	}
	return subtype
}

// total renders "Total Assets" / "Jumlah Harta".
func (c reportCopy) total(section string) string {
	return fmt.Sprintf("%s %s", c.totalPrefix, section)
}

var monthNamesEN = [...]string{"", "January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December"}

var monthNamesIDCat = [...]string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember"}
