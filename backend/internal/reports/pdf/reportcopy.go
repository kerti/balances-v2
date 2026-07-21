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
	cashIn         string
	cashOut        string
	income         string
	bySource       string // sub-heading for the active/passive income split
	activeIncome   string
	passiveIncome  string
	couponsPaidOut string // paid-out bond-coupon cash, additive line
	expenses       string
	netCashFlow    string
	joint          string

	totalPrefix string // "Total" / "Jumlah" — combined with a section name

	fxTitle string
	fxLine  string // 1 %[1]s = %[2]s %[3]s

	staleNote string // footnote for carried-forward values

	// financial-health panel (#412, ADR-0048)
	statNote       string   // section intro: trailing-12 smoothing + expenses-are-estimated
	statRows       []string // the four ratio labels, in render order
	statDescs      []string // parallel one-sentence explanation for each ratio
	statUndefined  string   // note shown when a ratio's inputs are unavailable
	statIndefinite string   // Fund Resilience value when the pool never depletes
	statYearUnit   string   // Fund Resilience value, singular ("%s year")
	statYearsUnit  string   // Fund Resilience value, plural ("%s years")
	statMonthUnit  string   // Fund Resilience value, singular ("%s month")
	statMonthsUnit string   // Fund Resilience value, plural ("%s months")

	// reproducibility block: the trailing-12 operands behind the two flow ratios
	statInputsTitle   string // sub-heading for the inputs block
	statInputIncome   string // avg monthly income (routine) label
	statInputExpenses string // avg monthly living expenses (estimated) label
	statInputPassive  string // avg monthly total passive income label
	statFormulaCash   string // Cash-Flow ratio formula, in words
	statFormulaPass   string // Passive-Income ratio formula, in words

	// investment-performance block (ADR-0048 amendment)
	investmentPerf  string // section title
	perfNote        string // intro: rate = return ÷ opening capital; trailing-12 compound
	perfColMonth    string // "This month" column header
	perfColTrailing string // "12-mo" column header
	perfTotal       string // "All investments" total-row label
	perfByRisk      string // sub-heading: by risk profile
	perfByType      string // sub-heading: by instrument type
	perfAmount      string // muted "%s this month" amount-context line
	perfPlacement   string // placement row label (new money in, % of pool)
	perfPlaceNote   string // muted note explaining placement + rollover exclusion

	deltaVs    string // "vs %s" — month-over-month comparison suffix
	footerPage string // "Page %d of %s" (%s = total-pages alias)

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
		bySource:         "By source",
		activeIncome:     "Active income",
		passiveIncome:    "Passive income",
		couponsPaidOut:   "Bond coupons paid out",
		expenses:         "Living Expenses (estimated)",
		netCashFlow:      "Net Cash Flow",
		joint:            "Joint",
		totalPrefix:      "Total",
		fxTitle:          "Exchange Rates Used",
		fxLine:           "1 %[1]s = %[2]s %[3]s",
		staleNote:        "* carried forward from an earlier month's statement",
		deltaVs:          "vs %s",
		footerPage:       "Page %d of %s",
		statNote:         "Income, living expenses and passive income are 12-month trailing averages, so one unusual month doesn't skew the picture. Balances — cash and investments — are read as of this month. Living expenses are never recorded directly: they're derived as what came in (income and investment returns) minus the change in your net worth, so every expense figure is an estimate.",
		statRows:         []string{"Cash-Flow Ratio", "Passive-Income Ratio", "Instant-Liquidity Ratio", "Fund Resilience"},
		statDescs: []string{
			"Share of earned income kept after living expenses, averaged over the past year. Counts income you marked as regular — one-offs like bonuses or severance are left out.",
			"Passive income — rent, pension, interest and investment returns — as a share of living expenses; 100% means it fully covers them. Only regular passive income counts. Living expenses are estimated, and this figure moves with the market.",
			"Cash held in bank accounts as a share of total investments. A ceiling gauge: a high figure can signal idle cash that could be put to work.",
			"How long investments would last if active income stopped, drawing estimated living expenses net of continuing passive income. Counts only income you marked as regular — one-offs like severance are excluded.",
		},
		statUndefined:     "Not enough history yet to calculate.",
		statIndefinite:    "Indefinite",
		statYearUnit:      "%s year",
		statYearsUnit:     "%s years",
		statMonthUnit:     "%s month",
		statMonthsUnit:    "%s months",
		statInputsTitle:   "Inputs — 12-mo avg, regular income only",
		statInputIncome:   "Avg income",
		statInputExpenses: "Avg living expenses",
		statInputPassive:  "Avg total passive income",
		statFormulaCash:   "Cash-Flow = (income − living expenses) ÷ income",
		statFormulaPass:   "Passive-Income = total passive income ÷ living expenses",
		investmentPerf:    "Investment Performance",
		perfNote:          "Return as a rate — this month's investment return over the value you started the month holding. The 12-month figure compounds the last year's monthly returns, so it reads as the trend rather than one lumpy month. A bucket you held for the first month, or have fully exited, has no starting value to measure against and shows \"—\". The three breakdowns don't add up to the total: each rate is measured against its own base.",
		perfColMonth:      "This month",
		perfColTrailing:   "12-mo",
		perfTotal:         "All investments",
		perfByRisk:        "By risk profile",
		perfByType:        "By instrument type",
		perfAmount:        "%s this month",
		perfPlacement:     "New money placed",
		perfPlaceNote:     "New money you deployed into investments, as a share of the pool it added to — this month beside its 12-month average. Only fresh money from your accounts counts: rollovers and reinvested principal are excluded.",
		chartAssets:       "Assets Composition",
		chartInvestments:  "Investments Composition",
		chartLiabilities:  "Liabilities Composition",
		chartTrend:        "Net Worth — last 12 months",
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
		bySource:         "Berdasarkan sumber",
		activeIncome:     "Pendapatan aktif",
		passiveIncome:    "Pendapatan pasif",
		couponsPaidOut:   "Kupon obligasi dibayarkan",
		expenses:         "Pengeluaran (estimasi)",
		netCashFlow:      "Arus Kas Bersih",
		joint:            "Bersama",
		totalPrefix:      "Jumlah",
		fxTitle:          "Kurs yang Digunakan",
		fxLine:           "1 %[1]s = %[2]s %[3]s",
		staleNote:        "* nilai dibawa dari laporan bulan sebelumnya",
		deltaVs:          "vs %s",
		footerPage:       "Halaman %d dari %s",
		statNote:         "Pendapatan, pengeluaran, dan pendapatan pasif memakai rata-rata 12 bulan terakhir, agar satu bulan yang tidak biasa tidak membiaskan gambaran. Saldo — kas dan investasi — dibaca per bulan ini. Pengeluaran tidak pernah dicatat langsung: angkanya diturunkan dari uang yang masuk (pendapatan dan imbal hasil investasi) dikurangi perubahan kekayaan bersih, sehingga setiap angka pengeluaran adalah estimasi.",
		statRows:         []string{"Rasio Arus Kas", "Rasio Pendapatan Pasif", "Rasio Likuiditas Instan", "Ketahanan Dana"},
		statDescs: []string{
			"Bagian dari pendapatan yang disisihkan setelah pengeluaran, dirata-ratakan selama setahun terakhir. Hanya menghitung pendapatan yang Anda tandai rutin — pemasukan sekali waktu seperti bonus atau pesangon tidak disertakan.",
			"Pendapatan pasif — sewa, pensiun, bunga dan imbal hasil investasi — sebagai bagian dari pengeluaran; 100% berarti menutupi seluruhnya. Hanya pendapatan pasif rutin yang dihitung. Pengeluaran bersifat perkiraan, dan angka ini bergerak mengikuti pasar.",
			"Kas di rekening bank sebagai bagian dari total investasi. Tolok ukur batas atas: angka tinggi menandakan kas menganggur yang bisa didayagunakan.",
			"Berapa lama investasi akan bertahan bila pendapatan aktif berhenti, dikurangi perkiraan pengeluaran setelah dipotong pendapatan pasif yang terus berjalan. Hanya menghitung pendapatan yang Anda tandai rutin — pemasukan sekali waktu seperti pesangon tidak disertakan.",
		},
		statUndefined:     "Riwayat belum cukup untuk dihitung.",
		statIndefinite:    "Tak terbatas",
		statYearUnit:      "%s tahun",
		statYearsUnit:     "%s tahun",
		statMonthUnit:     "%s bulan",
		statMonthsUnit:    "%s bulan",
		statInputsTitle:   "Masukan — rata-rata 12 bln, pendapatan rutin saja",
		statInputIncome:   "Rata-rata pendapatan",
		statInputExpenses: "Rata-rata pengeluaran",
		statInputPassive:  "Rata-rata total pendapatan pasif",
		statFormulaCash:   "Arus Kas = (pendapatan − pengeluaran) ÷ pendapatan",
		statFormulaPass:   "Pendapatan Pasif = total pendapatan pasif ÷ pengeluaran",
		investmentPerf:    "Kinerja Investasi",
		perfNote:          "Imbal hasil sebagai tingkat — imbal hasil investasi bulan ini dibagi nilai yang Anda pegang di awal bulan. Angka 12 bulan menggabungkan imbal hasil bulanan setahun terakhir secara majemuk, sehingga terbaca sebagai tren, bukan satu bulan yang menonjol. Aset yang baru dipegang bulan pertama, atau sudah dilepas seluruhnya, tidak punya nilai awal sebagai pembanding dan ditampilkan \"—\". Ketiga rincian tidak menjumlah ke total: setiap tingkat diukur terhadap basisnya sendiri.",
		perfColMonth:      "Bulan ini",
		perfColTrailing:   "12-bln",
		perfTotal:         "Seluruh investasi",
		perfByRisk:        "Berdasarkan profil risiko",
		perfByType:        "Berdasarkan jenis instrumen",
		perfAmount:        "%s bulan ini",
		perfPlacement:     "Dana baru ditempatkan",
		perfPlaceNote:     "Dana baru yang Anda tempatkan ke investasi, sebagai bagian dari pool yang ditambahnya — bulan ini di samping rata-rata 12 bulannya. Hanya dana baru dari rekening Anda yang dihitung: perpanjangan (rollover) dan penempatan ulang pokok tidak disertakan.",
		chartAssets:       "Komposisi Harta",
		chartInvestments:  "Komposisi Investasi",
		chartLiabilities:  "Komposisi Hutang",
		chartTrend:        "Kekayaan Bersih — 12 Bulan Terakhir",
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

// riskLabels maps an Investment risk_profile to its localized display name
// (ADR-0048 amendment — investment-performance breakdown).
var riskLabels = map[string]map[string]string{
	"en-GB": {"low": "Low risk", "medium": "Medium risk", "high": "High risk"},
	"id-ID": {"low": "Risiko rendah", "medium": "Risiko menengah", "high": "Risiko tinggi"},
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

func riskLabel(locale, risk string) string {
	if m, ok := riskLabels[locale]; ok {
		if s, ok := m[risk]; ok {
			return s
		}
	}
	if s, ok := riskLabels[fallbackLocale][risk]; ok {
		return s
	}
	return risk
}

// total renders "Total Assets" / "Jumlah Harta".
func (c reportCopy) total(section string) string {
	return fmt.Sprintf("%s %s", c.totalPrefix, section)
}

var monthNamesEN = [...]string{"", "January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December"}

var monthNamesIDCat = [...]string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember"}
