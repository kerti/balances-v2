package pdf

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

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

	// write-offs (ADR-0052): value that left the book with no cash settling it
	writeOffsTitle string
	writeOffsNote  string // one-sentence explanation for a non-technical reader
	writeOffsTotal string
	unsettledNote  string // footnote: investments terminated with no proceeds recorded

	// tracking changes (ADR-0053): value that crossed the edge of the book
	trackingChangesTitle string
	trackingChangesNote  string // one-sentence explanation for a non-technical reader
	trackingChangesTotal string

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
		title:            "Monthly financial report",
		subtitle:         "Figures reflect the household's finances on the last day of %s.",
		netWorth:         "Net worth",
		baseline:         "First reported month — no prior month to compare against.",
		statistics:       "Statistics",
		assets:           "Assets",
		liabilities:      "Liabilities",
		investments:      "Investments",
		receivables:      "Receivables",
		cashFlow:         "Cash flow",
		charts:           "Charts",
		currentAssets:    "Current assets",
		nonCurrentAssets: "Non-current assets",
		cashIn:           "Cash in",
		cashOut:          "Cash out",
		income:           "Income",
		bySource:         "By source",
		activeIncome:     "Active income",
		passiveIncome:    "Passive income",
		couponsPaidOut:   "Bond coupons paid out",
		expenses:         "Living expenses (estimated)",
		netCashFlow:      "Net cash flow",
		joint:            "Joint",
		totalPrefix:      "Total",
		fxTitle:          "Exchange rates used",
		fxLine:           "1 %[1]s = %[2]s %[3]s",
		staleNote:        "* carried forward from an earlier month's statement",
		writeOffsTitle:   "Write-offs",
		writeOffsNote:    "Value that left your books this month without any money changing hands — property or belongings disposed of, a debt someone forgave, money owed to you that won't be repaid. A forgiven debt counts in your favour, so the total can be positive.",
		writeOffsTotal:   "Total write-offs",
		unsettledNote:    "* terminated with no sale or maturity recorded, so its final value shows as an investment loss. If money did come back, record the sale for that month.",

		trackingChangesTitle: "Tracking changes",
		trackingChangesNote:  "Value that came into or went out of what you track here this month without being earned, spent or invested — something you already owned but only started tracking now, or an account that joined or left the household. It is neither income nor spending, so it is kept out of both.",
		trackingChangesTotal: "Total tracking changes",
		deltaVs:              "vs %s",
		footerPage:           "Page %d of %s",
		statNote:             "Income, living expenses and passive income are 12-month trailing averages, so one unusual month doesn't skew the picture. Balances — cash and investments — are read as of this month. Living expenses are never recorded directly: they're derived as what came in (income and investment returns) minus the change in your net worth, so every expense figure is an estimate.",
		statRows:             []string{"Cash-flow ratio", "Passive-income ratio", "Instant-liquidity ratio", "Fund resilience"},
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
		statFormulaCash:   "Cash-flow = (income − living expenses) ÷ income",
		statFormulaPass:   "Passive-income = total passive income ÷ living expenses",
		investmentPerf:    "Investment performance",
		perfNote:          "Return as a rate — this month's investment return over the value you started the month with. The 12-month figure compounds the last year's monthly returns, so it reads as the trend rather than one lumpy month. A bucket you held for the first month, or have fully exited, has no starting value to measure against and shows \"—\". The three breakdowns don't add up to the total: each rate is measured against its own base.",
		perfColMonth:      "This month",
		perfColTrailing:   "12-mo",
		perfTotal:         "All investments",
		perfByRisk:        "By risk profile",
		perfByType:        "By instrument type",
		perfAmount:        "%s this month",
		perfPlacement:     "Net money placed",
		perfPlaceNote:     "New money you put into investments, net of what came back out (sales and matured principal), as a share of the pool — this month beside its 12-month average. Reinvesting a matured bond or TD nets to about zero; a month you took out more than you put in shows negative. Coupons and dividends don't count here.",
		chartAssets:       "Assets composition",
		chartInvestments:  "Investments composition",
		chartLiabilities:  "Liabilities composition",
		chartTrend:        "Net worth — last 12 months",
	},
	"id-ID": {
		title:            "Laporan keuangan bulanan",
		subtitle:         "Data menunjukkan kondisi keuangan rumah tangga pada hari terakhir bulan %s.",
		netWorth:         "Kekayaan bersih",
		baseline:         "Bulan pertama yang dilaporkan — belum ada bulan sebelumnya untuk dibandingkan.",
		statistics:       "Statistika",
		assets:           "Aset",
		liabilities:      "Liabilitas",
		investments:      "Investasi",
		receivables:      "Piutang",
		cashFlow:         "Arus kas",
		charts:           "Grafik",
		currentAssets:    "Aset lancar",
		nonCurrentAssets: "Aset tidak lancar",
		cashIn:           "Kas masuk",
		cashOut:          "Kas keluar",
		income:           "Pemasukan",
		bySource:         "Berdasarkan sumber",
		activeIncome:     "Pemasukan aktif",
		passiveIncome:    "Pemasukan pasif",
		couponsPaidOut:   "Kupon obligasi dibayarkan",
		expenses:         "Pengeluaran (perkiraan)",
		netCashFlow:      "Arus kas bersih",
		joint:            "Bersama",
		totalPrefix:      "Jumlah",
		fxTitle:          "Kurs yang digunakan",
		fxLine:           "1 %[1]s = %[2]s %[3]s",
		staleNote:        "* nilai dibawa dari laporan bulan sebelumnya",
		writeOffsTitle:   "Penghapusan nilai",
		writeOffsNote:    "Nilai yang hilang dari pembukuan bulan ini tanpa ada uang yang berpindah — harta yang dilepas atau dibuang, utang yang dibebaskan pemberi pinjaman, atau piutang yang tidak akan tertagih. Utang yang dibebaskan menguntungkan Anda, jadi totalnya bisa bernilai positif.",
		writeOffsTotal:   "Jumlah penghapusan nilai",
		unsettledNote:    "* diakhiri tanpa pencatatan penjualan atau jatuh tempo, sehingga nilai akhirnya tampak sebagai kerugian investasi. Jika dananya kembali, catat penjualannya pada bulan tersebut.",

		trackingChangesTitle: "Perubahan cakupan pencatatan",
		trackingChangesNote:  "Nilai yang masuk atau keluar dari yang Anda catat di sini pada bulan ini tanpa diperoleh, dibelanjakan, atau diinvestasikan — sesuatu yang sudah Anda miliki tetapi baru mulai dicatat sekarang, atau rekening yang bergabung atau keluar dari rumah tangga. Ini bukan penghasilan maupun pengeluaran, jadi tidak dihitung sebagai keduanya.",
		trackingChangesTotal: "Jumlah perubahan cakupan pencatatan",
		deltaVs:              "vs %s",
		footerPage:           "Halaman %d dari %s",
		statNote:             "Pemasukan, pengeluaran, dan pemasukan pasif memakai rata-rata 12 bulan terakhir, agar satu bulan yang tidak biasa tidak membiaskan gambaran. Saldo — kas dan investasi — dibaca per bulan ini. Pengeluaran tidak pernah dicatat langsung: angkanya diturunkan dari uang yang masuk (pemasukan dan hasil investasi) dikurangi perubahan kekayaan bersih, sehingga setiap angka pengeluaran adalah estimasi.",
		statRows:             []string{"Rasio arus kas", "Rasio pemasukan pasif", "Rasio likuiditas instan", "Ketahanan dana"},
		statDescs: []string{
			"Bagian dari pemasukan yang disisihkan setelah pengeluaran, dirata-ratakan selama setahun terakhir. Hanya menghitung pemasukan yang Anda tandai Rutin — pemasukan Sekali waktu seperti bonus atau pesangon tidak disertakan.",
			"Pemasukan pasif — sewa, pensiun, bunga dan hasil investasi — sebagai bagian dari pengeluaran; 100% berarti menutupi seluruhnya. Hanya pemasukan pasif rutin yang dihitung. Pengeluaran bersifat perkiraan, dan angka ini bergerak mengikuti pasar.",
			"Kas di rekening bank sebagai bagian dari total investasi. Tolok ukur batas atas: angka tinggi menandakan kas menganggur yang bisa didayagunakan.",
			"Berapa lama investasi akan bertahan bila pemasukan aktif berhenti, dikurangi perkiraan pengeluaran setelah dipotong pemasukan pasif yang terus berjalan. Hanya menghitung pemasukan yang Anda tandai Rutin — pemasukan Sekali waktu seperti pesangon tidak disertakan.",
		},
		statUndefined:     "Riwayat belum cukup untuk dihitung.",
		statIndefinite:    "Tak terbatas",
		statYearUnit:      "%s tahun",
		statYearsUnit:     "%s tahun",
		statMonthUnit:     "%s bulan",
		statMonthsUnit:    "%s bulan",
		statInputsTitle:   "Dasar perhitungan — rata-rata 12 bln, pemasukan rutin saja",
		statInputIncome:   "Rata-rata pemasukan",
		statInputExpenses: "Rata-rata pengeluaran",
		statInputPassive:  "Rata-rata total pemasukan pasif",
		statFormulaCash:   "Arus kas = (pemasukan − pengeluaran) ÷ pemasukan",
		statFormulaPass:   "Pemasukan pasif = total pemasukan pasif ÷ pengeluaran",
		investmentPerf:    "Kinerja investasi",
		perfNote:          "Hasil sebagai tingkat — hasil investasi bulan ini dibagi nilai di awal bulan. Angka 12 bulan menggabungkan hasil bulanan setahun terakhir secara majemuk, sehingga terbaca sebagai tren, bukan satu bulan yang menonjol. Kelompok yang baru dimiliki bulan pertama, atau sudah dilepas seluruhnya, tidak punya nilai awal sebagai pembanding dan ditampilkan \"—\". Ketiga rincian tidak menjumlah ke total: setiap tingkat diukur terhadap basisnya sendiri.",
		perfColMonth:      "Bulan ini",
		perfColTrailing:   "12-bln",
		perfTotal:         "Seluruh investasi",
		perfByRisk:        "Berdasarkan profil risiko",
		perfByType:        "Berdasarkan jenis instrumen",
		perfAmount:        "%s bulan ini",
		perfPlacement:     "Dana bersih ditempatkan",
		perfPlaceNote:     "Dana baru yang Anda tempatkan ke investasi, dikurangi yang kembali keluar (penjualan dan pokok jatuh tempo), sebagai bagian dari pool — bulan ini di samping rata-rata 12 bulannya. Menempatkan ulang obligasi atau deposito yang jatuh tempo bersih mendekati nol; bulan saat Anda menarik lebih banyak daripada menempatkan akan bernilai negatif. Kupon dan dividen tidak dihitung di sini.",
		chartAssets:       "Komposisi aset",
		chartInvestments:  "Komposisi investasi",
		chartLiabilities:  "Komposisi liabilitas",
		chartTrend:        "Kekayaan bersih — 12 bulan terakhir",
	},
}

// subtypeLabels maps a domain subtype to its localized display name.
var subtypeLabels = map[string]map[string]string{
	"en-GB": {
		"bank_account":  "Bank accounts",
		"property":      "Property",
		"vehicle":       "Vehicles",
		"institutional": "Institutional liabilities",
		"personal":      "Personal liabilities",
		"stock":         "Stocks",
		"mutual_fund":   "Mutual funds",
		"bond":          "Bonds",
		"gold":          "Gold",
		"time_deposit":  "Time deposits",
	},
	"id-ID": {
		"bank_account":  "Rekening bank",
		"property":      "Properti",
		"vehicle":       "Kendaraan",
		"institutional": "Liabilitas institusional",
		"personal":      "Liabilitas pribadi",
		"stock":         "Saham",
		"mutual_fund":   "Reksa dana",
		"bond":          "Obligasi",
		"gold":          "Emas",
		"time_deposit":  "Deposito",
	},
}

// riskLabels maps an Investment risk_profile to its localized display name
// (ADR-0048 amendment — investment-performance breakdown).
var riskLabels = map[string]map[string]string{
	"en-GB": {"low": "Low risk", "medium": "Medium risk", "high": "High risk"},
	"id-ID": {"low": "Risiko rendah", "medium": "Risiko sedang", "high": "Risiko tinggi"},
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

// total renders "Total assets" / "Jumlah aset" — sentence case, so the section
// name is lowercased after the prefix.
func (c reportCopy) total(section string) string {
	r, n := utf8.DecodeRuneInString(section)
	return fmt.Sprintf("%s %c%s", c.totalPrefix, unicode.ToLower(r), section[n:])
}

// totalOf is the "Total <name>" label for a proper name (a household member),
// which keeps its capital.
func (c reportCopy) totalOf(name string) string {
	return c.totalPrefix + " " + name
}

var monthNamesEN = [...]string{"", "January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December"}

var monthNamesIDCat = [...]string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember"}
