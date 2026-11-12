package backup

import (
	"time"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

// The demo household's hand-written history (ADR-0041). Every figure below is
// invented; none of it is, or derives from, a real household's data.
//
// The tables are indexed by calendar month and keyed by years back from the
// current year, so seasonality stays on real calendar months even though the
// 72-month window rolls forward every night (see demoSlot). A row reads
// January..December of one year; the oldest row is partly unused (the window
// starts mid-year) and the newest only up to the current month, but every cell
// is filled in so any month can become "now" without a gap.
//
// The household in brief: a Jakarta family of two. One salaried earner whose
// take-home steps up every January, a second member on a small pension and
// freelance work, a mortgage, a rental flat and a few investments. The Eid
// bonus (THR) lands in April at a different size each year, school fees fall in
// July, there is a year-end bonus most years and a holiday dip some Decembers.
// The Family Home is revalued in January (once by a lot, when a toll-road
// interchange opened nearby), the Rental Apartment each July. Markets slid from
// February to April of "1 year ago" and were back by August. Not everything is
// current or open: a few positions are not updated this month (demoNotUpdated),
// an old hatchback and a stock were sold and closed, one time deposit matured and
// rolled into a successor and another matured and was paid out. The checking
// balance is written to reconcile with everything else (INV-FINANCE-28): the
// living expenses the report engine derives from these tables are plausible and
// seasonal in every month, which the tests pin for the first and the last
// possible window.
//
// Units: IDR in whole rupiah; USD tables in cents; the inflation table in
// hundredths of a percent; the zero value means "not held / not recorded" in
// the tables where a position is born or a series starts mid-history.

// demoYearRows is the number of year rows in every table: six years back to
// the current year. A 72-month window ending in the current month touches the
// current year (January..now) and the six years before it only partly in the
// oldest row.
const demoYearRows = 7

// demoGrid is one table: [years back, oldest first][calendar month, January
// first]. Row r is "demoYearRows-1-r years ago".
type demoGrid [demoYearRows][12]int64

// demoWhen places an event on the same calendar: yearsBack years before the
// current year, in the given month. Negative yearsBack is the future (a
// maturity date).
type demoWhen struct {
	yearsBack int
	month     time.Month
}

func at(yearsBack int, month time.Month) demoWhen { return demoWhen{yearsBack, month} }

type demoOwner int

const (
	demoOwnerSole    demoOwner = iota // the demo user
	demoOwnerMember2                  // the second household member
	demoOwnerJoint
)

// demoIncomeEvent is one Income row that is not a plain monthly figure.
type demoIncomeEvent struct {
	when        demoWhen
	day         int
	category    string
	description string
	amount      int64
	owner       demoOwner
	regularity  string
}

// demoTrade is a Buy (qty > 0) or Sell (qty < 0), in whole units.
type demoTrade struct {
	when demoWhen
	qty  int64
}

// demoCashEvent is an amount-only ledger entry (no quantity or price).
type demoCashEvent struct {
	when   demoWhen
	day    int
	txn    string
	amount int64
}

// demoClosing is how a position left the portfolio: the terminal lifecycle
// flip the app's Terminate dialog makes (ADR-0052), dated on a day of a month.
type demoClosing struct {
	when demoWhen
	day  int
	note string
}

// demoMarket is a stock, fund or gold position: its price table, the trades
// that build the holding and the payouts and fees along the way. closing, when
// set, is the terminal Sell that closes the whole holding: the position is
// recorded up to the month before, and the lifecycle flip writes the Sell at
// that month's price and the zero close snapshot.
type demoMarket struct {
	prices  demoGrid
	trades  []demoTrade
	cash    []demoCashEvent
	closing *demoClosing
}

// demoClosedAsset is a vehicle that was sold: its value table runs up to the
// month before the sale (zero from the sale on) and the lifecycle flip to "sold"
// writes the zero close snapshot.
type demoClosedAsset struct {
	closing demoClosing
	value   demoGrid
}

// demoMaturedDeposit is a time deposit whose term ended inside the history. It
// is recorded up to the month before its maturity date (placed + termMonths);
// at maturity the principal and interest either rolled into a successor deposit
// or were paid out to the bank. value is the dirty total (principal + accrued).
type demoMaturedDeposit struct {
	placed     demoWhen
	termMonths int
	principal  int64
	interest   int64
	value      demoGrid
}

// demoBond is a bond position bought mid-history. value is the dirty total
// (it includes accrued); coupons is the coupon cash received, per month; matures
// is in the future (negative yearsBack) so the bond is still running.
type demoBond struct {
	placed  demoWhen
	matures demoWhen
	value   demoGrid
	accrued demoGrid
	coupons demoGrid
}

// demoDeposit is a time deposit placed mid-history; value is the dirty total.
// The term must still be running (a deposit's snapshots fall inside its term).
type demoDeposit struct {
	placed     demoWhen
	termMonths int
	value      demoGrid
}

// ---- Income ----------------------------------------------------------------

// demoSalary is the demo user's monthly salary, paid on the 25th (routine). A
// month now and then sits above the base on overtime or an allowance.
var demoSalary = demoGrid{
	// 6 years ago: 5.7M take-home
	{5_700_000, 5_850_000, 5_700_000, 5_700_000, 5_900_000, 5_700_000, 5_700_000, 5_700_000, 5_700_000, 5_700_000, 5_700_000, 5_850_000},
	// 5 years ago: 5.9M take-home, raised in January
	{5_900_000, 5_900_000, 5_900_000, 5_900_000, 5_900_000, 6_320_000, 6_050_000, 5_900_000, 5_900_000, 5_900_000, 5_900_000, 5_900_000},
	// 4 years ago: 6.15M take-home, raised in January
	{6_150_000, 6_150_000, 6_150_000, 6_150_000, 6_150_000, 6_150_000, 6_150_000, 6_150_000, 6_570_000, 6_400_000, 6_150_000, 6_150_000},
	// 3 years ago: 6.35M take-home, raised in January
	{6_350_000, 6_350_000, 6_350_000, 6_350_000, 6_550_000, 6_350_000, 6_350_000, 6_350_000, 6_500_000, 6_770_000, 6_650_000, 6_350_000},
	// 2 years ago: 6.55M take-home, raised in January
	{6_550_000, 6_550_000, 6_550_000, 6_550_000, 6_850_000, 6_550_000, 6_550_000, 6_550_000, 6_550_000, 6_550_000, 6_550_000, 6_800_000},
	// 1 year ago: 6.75M take-home, raised in January
	{6_750_000, 6_750_000, 6_750_000, 6_750_000, 6_750_000, 6_900_000, 6_750_000, 7_000_000, 6_750_000, 6_750_000, 6_750_000, 6_750_000},
	// this year: 7M take-home, raised in January
	{7_000_000, 7_000_000, 7_000_000, 7_000_000, 7_000_000, 7_000_000, 7_300_000, 7_200_000, 7_000_000, 7_000_000, 7_000_000, 7_000_000},
}

// demoPension is the second member's monthly pension, paid on the 3rd (routine).
var demoPension = demoGrid{
	// 6 years ago: 1.8M a month
	{1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000},
	// 5 years ago: 1.8M a month, unchanged
	{1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000, 1_800_000},
	// 4 years ago: 1.95M a month, indexed up in January
	{1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000},
	// 3 years ago: 1.95M a month, unchanged
	{1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000},
	// 2 years ago: 2.1M a month, indexed up in January
	{2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000},
	// 1 year ago: 2.1M a month, unchanged
	{2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000, 2_100_000},
	// this year: 2.25M a month, indexed up in January
	{2_250_000, 2_250_000, 2_250_000, 2_250_000, 2_250_000, 2_250_000, 2_250_000, 2_250_000, 2_250_000, 2_250_000, 2_250_000, 2_250_000},
}

// demoSavingsInterest is the interest credited on Joint Savings, on the 25th
// (incidental: negligible, and it stays in the account that earned it).
var demoSavingsInterest = demoGrid{
	// 6 years ago: grows with the Joint Savings balance
	{25_000, 29_000, 34_000, 35_000, 46_000, 46_000, 53_000, 55_000, 61_000, 66_000, 69_000, 65_000},
	// 5 years ago: keeps growing
	{77_000, 78_000, 75_000, 43_000, 53_000, 49_000, 56_000, 65_000, 60_000, 67_000, 58_000, 54_000},
	// 4 years ago: keeps growing
	{57_000, 60_000, 58_000, 63_000, 58_000, 64_000, 50_000, 42_000, 42_000, 42_000, 45_000, 61_000},
	// 3 years ago: levels off
	{61_000, 62_000, 58_000, 34_000, 36_000, 40_000, 51_000, 54_000, 128_000, 132_000, 153_000, 147_000},
	// 2 years ago: falls from October, after 20M left Joint Savings for the time deposit
	{157_000, 152_000, 154_000, 147_000, 154_000, 172_000, 178_000, 187_000, 178_000, 166_000, 146_000, 151_000},
	// 1 year ago: recovering
	{155_000, 152_000, 159_000, 156_000, 166_000, 137_000, 144_000, 147_000, 153_000, 146_000, 132_000, 156_000},
	// this year: recovering
	{139_000, 161_000, 154_000, 180_000, 155_000, 191_000, 194_000, 170_000, 178_000, 203_000, 205_000, 202_000},
}

// demoIncomeEvents are the one-off and seasonal Income rows. THR (the Eid bonus)
// lands in April and the year-end bonus in December, at a different size each
// year; the rest spread every other category across the history so the income
// breakdown has variety. Monthly salary, pension and interest are the tables above.
var demoIncomeEvents = []demoIncomeEvent{
	// 6 years ago
	{at(6, time.April), 15, "salary", "THR (religious holiday bonus)", 6_200_000, demoOwnerSole, "incidental"},
	{at(6, time.August), 15, "business_income", "Side consulting project", 1_400_000, demoOwnerSole, "incidental"},
	// 5 years ago
	{at(5, time.April), 15, "salary", "THR (religious holiday bonus)", 6_450_000, demoOwnerSole, "incidental"},
	{at(5, time.July), 15, "tax_refund", "Annual tax refund", 320_000, demoOwnerMember2, "incidental"},
	{at(5, time.November), 15, "other", "Marketplace resale", 180_000, demoOwnerMember2, "incidental"},
	{at(5, time.December), 20, "salary", "Year-end bonus", 1_500_000, demoOwnerSole, "incidental"},
	// 4 years ago
	{at(4, time.February), 15, "rental_income", "Spare room rental income", 600_000, demoOwnerJoint, "incidental"},
	{at(4, time.April), 15, "salary", "THR (religious holiday bonus)", 6_700_000, demoOwnerSole, "incidental"},
	{at(4, time.September), 15, "gift", "Wedding gift from relatives", 1_000_000, demoOwnerJoint, "incidental"},
	// 3 years ago
	{at(3, time.April), 15, "salary", "THR (religious holiday bonus)", 6_150_000, demoOwnerSole, "incidental"},
	{at(3, time.June), 15, "insurance_payout", "Health insurance reimbursement", 1_150_000, demoOwnerSole, "incidental"},
	{at(3, time.July), 15, "tax_refund", "Annual tax refund", 410_000, demoOwnerMember2, "incidental"},
	{at(3, time.October), 15, "business_income", "Side consulting project", 2_100_000, demoOwnerSole, "incidental"},
	{at(3, time.December), 20, "salary", "Year-end bonus", 2_000_000, demoOwnerSole, "incidental"},
	// 2 years ago
	{at(2, time.March), 15, "rental_income", "Spare room rental income", 750_000, demoOwnerJoint, "incidental"},
	{at(2, time.April), 15, "salary", "THR (religious holiday bonus)", 7_300_000, demoOwnerSole, "incidental"},
	{at(2, time.August), 15, "other", "Marketplace resale", 240_000, demoOwnerMember2, "incidental"},
	{at(2, time.December), 15, "gift", "Holiday gift from family", 500_000, demoOwnerJoint, "incidental"},
	{at(2, time.December), 20, "salary", "Year-end bonus", 2_400_000, demoOwnerSole, "incidental"},
	// 1 year ago
	{at(1, time.January), 15, "rental_income", "Spare room rental income", 750_000, demoOwnerJoint, "incidental"},
	{at(1, time.April), 15, "salary", "THR (religious holiday bonus)", 7_600_000, demoOwnerSole, "incidental"},
	{at(1, time.July), 15, "tax_refund", "Annual tax refund", 460_000, demoOwnerMember2, "incidental"},
	{at(1, time.November), 15, "insurance_payout", "Health insurance reimbursement", 900_000, demoOwnerSole, "incidental"},
	// this year
	{at(0, time.February), 15, "business_income", "Side consulting project", 1_600_000, demoOwnerSole, "incidental"},
	{at(0, time.April), 15, "salary", "THR (religious holiday bonus)", 7_950_000, demoOwnerSole, "incidental"},
	{at(0, time.May), 15, "rental_income", "Spare room rental income", 800_000, demoOwnerJoint, "incidental"},
	{at(0, time.July), 15, "tax_refund", "Annual tax refund", 480_000, demoOwnerMember2, "incidental"},
	{at(0, time.September), 15, "gift", "Wedding gift from relatives", 1_000_000, demoOwnerJoint, "incidental"},
	{at(0, time.December), 20, "salary", "Year-end bonus", 2_800_000, demoOwnerSole, "incidental"},
}

// ---- Bank accounts ---------------------------------------------------------

// demoChecking is the Everyday Checking balance. It is the household's
// working cash, written to reconcile with every other table: income in, living
// expenses, investment purchases and payouts, debt payments and savings
// transfers all net out to it (INV-FINANCE-28).
var demoChecking = demoGrid{
	// 6 years ago: ranges 12.7M to 18.6M; what is left above that is swept to Joint Savings
	{15_200_000, 14_081_000, 15_040_000, 16_510_000, 18_638_000, 15_211_000, 12_697_000, 14_012_000, 13_569_000, 14_797_000, 17_154_000, 12_772_000},
	// 5 years ago: ranges 9.7M to 17.2M
	{14_615_000, 16_819_000, 16_628_000, 16_333_000, 16_420_000, 17_238_000, 14_453_000, 15_638_000, 12_116_000, 9_682_000, 12_026_000, 14_864_000},
	// 4 years ago: ranges 9.6M to 17.8M; the April mortgage prepayment comes out of the THR
	{15_249_000, 17_812_000, 17_341_000, 12_173_000, 12_232_000, 9_555_000, 11_046_000, 12_635_000, 14_389_000, 16_509_000, 13_236_000, 12_474_000},
	// 3 years ago: ranges 10.0M to 23.1M; August peaks when the Old Hatchback's sale proceeds land
	{13_959_000, 13_874_000, 12_086_000, 13_624_000, 9_992_000, 17_244_000, 13_696_000, 23_066_000, 12_501_000, 15_681_000, 18_284_000, 14_800_000},
	// 2 years ago: ranges 10.2M to 18.3M; ORI024 (April) and the Mandiri deposit (September) are both funded around it
	{13_446_000, 15_221_000, 17_780_000, 10_249_000, 12_444_000, 17_509_000, 16_416_000, 18_315_000, 12_038_000, 11_973_000, 13_863_000, 13_824_000},
	// 1 year ago: ranges 11.1M to 17.5M; buying the dip in April and May comes out of it
	{16_299_000, 13_999_000, 15_531_000, 14_410_000, 11_962_000, 12_466_000, 11_123_000, 12_817_000, 13_409_000, 12_289_000, 11_568_000, 17_509_000},
	// this year: ranges 12.4M to 19.3M
	{16_051_000, 17_018_000, 16_980_000, 19_311_000, 18_988_000, 19_141_000, 15_246_000, 13_309_000, 14_163_000, 12_358_000, 14_234_000, 15_790_000},
}

// demoJointSavings is the Joint Savings balance (Emergency Fund tag).
var demoJointSavings = demoGrid{
	// 6 years ago: 13.0M in January, 35.8M by December: what the salary leaves is moved over in uneven amounts
	{13_000_000, 16_500_000, 16_500_000, 21_200_000, 21_200_000, 26_400_000, 28_700_000, 30_000_000, 30_000_000, 30_000_000, 30_000_000, 35_800_000},
	// 5 years ago: 35.8M in January, 26.6M by December; March, the BNI time deposit takes 15M (-13.4M net of the month's top-up)
	{35_800_000, 35_800_000, 22_400_000, 24_900_000, 24_900_000, 26_700_000, 28_400_000, 28_400_000, 29_800_000, 29_800_000, 26_550_000, 26_550_000},
	// 4 years ago: 28.1M in January, 27.1M by December; June, the Danamon time deposit takes 10M (-10.0M net)
	{28_150_000, 28_150_000, 28_150_000, 29_850_000, 31_950_000, 21_950_000, 21_950_000, 21_950_000, 21_950_000, 21_950_000, 27_050_000, 27_050_000},
	// 3 years ago: 27.1M in January, 75.6M by December; March (-9.2M) pays for the Astra Sedaya bond; June, the Danamon proceeds come back (+6.3M); August (+29.9M) is mostly the Old Hatchback's sale
	{27_050_000, 27_050_000, 17_800_000, 17_800_000, 17_800_000, 24_100_000, 27_200_000, 57_100_000, 66_400_000, 66_400_000, 66_400_000, 75_600_000},
	// 2 years ago: 75.6M in January, 67.2M by December; March, the BNI deposit rolls over without touching savings; June (+10.5M) is the TLKM sale; September, the Mandiri deposit takes 20M (-13.4M net), October -5.5M
	{75_600_000, 75_600_000, 75_600_000, 75_600_000, 75_600_000, 86_100_000, 86_100_000, 86_100_000, 72_700_000, 67_200_000, 67_200_000, 67_200_000},
	// 1 year ago: 67.2M in January, 69.0M by December; April +5.6M; May (-7.2M) goes into buying the dip
	{67_200_000, 69_000_000, 69_000_000, 74_600_000, 67_350_000, 67_350_000, 67_350_000, 67_350_000, 67_350_000, 68_950_000, 68_950_000, 68_950_000},
	// this year: 72.2M in January, 94.5M by December
	{72_150_000, 78_050_000, 79_850_000, 82_050_000, 84_150_000, 84_150_000, 87_050_000, 90_750_000, 90_750_000, 92_650_000, 92_650_000, 94_450_000},
}

// demoUSDSavings is the USD Savings balance, in cents (second member's account).
var demoUSDSavings = demoGrid{
	// 6 years ago: starts at $1,200, small top-ups
	{120_000, 120_000, 128_000, 128_000, 128_000, 130_500, 130_500, 130_500, 136_500, 136_500, 136_500, 136_500},
	// 5 years ago: $120 out in December for the holiday
	{140_500, 140_500, 140_500, 140_500, 151_500, 151_500, 154_500, 154_500, 154_500, 154_500, 163_500, 151_500},
	// 4 years ago: $150 out in July for a family trip
	{151_500, 151_500, 156_500, 156_500, 156_500, 163_500, 148_500, 148_500, 148_500, 151_500, 151_500, 151_500},
	// 3 years ago: $250 out in December for the holiday
	{151_500, 163_500, 163_500, 163_500, 163_500, 167_500, 167_500, 167_500, 176_500, 176_500, 176_500, 151_500},
	// 2 years ago: top-ups build towards the deposit
	{161_500, 161_500, 165_000, 165_000, 165_000, 177_000, 177_000, 177_000, 177_000, 190_000, 193_000, 193_000},
	// 1 year ago: $1,500 moves into the BCA time deposit in March; $410 out in December for the holiday
	{193_000, 207_000, 57_000, 57_000, 57_000, 57_000, 61_500, 61_500, 61_500, 68_500, 68_500, 27_500},
	// this year: rebuilding
	{27_500, 27_500, 30_500, 30_500, 36_500, 36_500, 36_500, 36_500, 39_000, 48_000, 48_000, 48_000},
}

// ---- Property and vehicles -------------------------------------------------

// demoFamilyHome is the Family Home's appraised value. Revalued once a year, in
// January, and flat in between (4%/yr advertised rate).
var demoFamilyHome = demoGrid{
	// 6 years ago: appraised at 192.0M
	{192_000_000, 192_000_000, 192_000_000, 192_000_000, 192_000_000, 192_000_000, 192_000_000, 192_000_000, 192_000_000, 192_000_000, 192_000_000, 192_000_000},
	// 5 years ago: +3.9% in January
	{199_600_000, 199_600_000, 199_600_000, 199_600_000, 199_600_000, 199_600_000, 199_600_000, 199_600_000, 199_600_000, 199_600_000, 199_600_000, 199_600_000},
	// 4 years ago: +4.0% in January
	{207_500_000, 207_500_000, 207_500_000, 207_500_000, 207_500_000, 207_500_000, 207_500_000, 207_500_000, 207_500_000, 207_500_000, 207_500_000, 207_500_000},
	// 3 years ago: +9.0% in January: a toll-road interchange opened nearby
	{226_200_000, 226_200_000, 226_200_000, 226_200_000, 226_200_000, 226_200_000, 226_200_000, 226_200_000, 226_200_000, 226_200_000, 226_200_000, 226_200_000},
	// 2 years ago: +3.5% in January
	{234_000_000, 234_000_000, 234_000_000, 234_000_000, 234_000_000, 234_000_000, 234_000_000, 234_000_000, 234_000_000, 234_000_000, 234_000_000, 234_000_000},
	// 1 year ago: +4.0% in January
	{243_400_000, 243_400_000, 243_400_000, 243_400_000, 243_400_000, 243_400_000, 243_400_000, 243_400_000, 243_400_000, 243_400_000, 243_400_000, 243_400_000},
	// this year: +4.2% in January
	{253_600_000, 253_600_000, 253_600_000, 253_600_000, 253_600_000, 253_600_000, 253_600_000, 253_600_000, 253_600_000, 253_600_000, 253_600_000, 253_600_000},
}

// demoRentalApartment is the Rental Apartment's value. Revalued once a year, in
// July, and flat in between (3.5%/yr advertised rate).
var demoRentalApartment = demoGrid{
	// 6 years ago: 121M until July, 125M after the mid-year appraisal
	{121_000_000, 121_000_000, 121_000_000, 121_000_000, 121_000_000, 121_000_000, 125_000_000, 125_000_000, 125_000_000, 125_000_000, 125_000_000, 125_000_000},
	// 5 years ago: 125M until July, 129M after the mid-year appraisal
	{125_000_000, 125_000_000, 125_000_000, 125_000_000, 125_000_000, 125_000_000, 129_000_000, 129_000_000, 129_000_000, 129_000_000, 129_000_000, 129_000_000},
	// 4 years ago: 129M until July, 133M after the mid-year appraisal
	{129_000_000, 129_000_000, 129_000_000, 129_000_000, 129_000_000, 129_000_000, 133_000_000, 133_000_000, 133_000_000, 133_000_000, 133_000_000, 133_000_000},
	// 3 years ago: 133M until July, 137M after the mid-year appraisal
	{133_000_000, 133_000_000, 133_000_000, 133_000_000, 133_000_000, 133_000_000, 137_000_000, 137_000_000, 137_000_000, 137_000_000, 137_000_000, 137_000_000},
	// 2 years ago: 137M until July, 141.5M after the mid-year appraisal
	{137_000_000, 137_000_000, 137_000_000, 137_000_000, 137_000_000, 137_000_000, 141_500_000, 141_500_000, 141_500_000, 141_500_000, 141_500_000, 141_500_000},
	// 1 year ago: 141.5M until July, 146M after the mid-year appraisal
	{141_500_000, 141_500_000, 141_500_000, 141_500_000, 141_500_000, 141_500_000, 146_000_000, 146_000_000, 146_000_000, 146_000_000, 146_000_000, 146_000_000},
	// this year: 146M until July, 150.5M after the mid-year appraisal
	{146_000_000, 146_000_000, 146_000_000, 146_000_000, 146_000_000, 146_000_000, 150_500_000, 150_500_000, 150_500_000, 150_500_000, 150_500_000, 150_500_000},
}

// demoFamilyCar is the Family Car's value, shedding roughly 8% a year.
var demoFamilyCar = demoGrid{
	// 6 years ago: depreciating a little each month
	{100_000_000, 99_400_000, 98_900_000, 98_100_000, 97_600_000, 96_900_000, 96_400_000, 95_800_000, 95_000_000, 94_500_000, 93_800_000, 93_100_000},
	// 5 years ago: depreciating a little each month
	{92_400_000, 91_700_000, 91_300_000, 90_700_000, 90_200_000, 89_700_000, 89_200_000, 88_600_000, 88_100_000, 87_500_000, 86_700_000, 86_000_000},
	// 4 years ago: depreciating a little each month
	{85_400_000, 84_800_000, 84_200_000, 83_700_000, 83_400_000, 82_900_000, 82_300_000, 81_600_000, 80_900_000, 80_200_000, 79_500_000, 79_000_000},
	// 3 years ago: depreciating a little each month
	{78_400_000, 77_900_000, 77_400_000, 76_800_000, 76_300_000, 75_700_000, 75_100_000, 74_600_000, 74_000_000, 73_600_000, 73_100_000, 72_600_000},
	// 2 years ago: depreciating a little each month
	{71_900_000, 71_600_000, 71_100_000, 70_700_000, 70_200_000, 69_700_000, 69_300_000, 68_800_000, 68_400_000, 67_900_000, 67_400_000, 66_900_000},
	// 1 year ago: depreciating a little each month
	{66_500_000, 65_900_000, 65_500_000, 65_000_000, 64_600_000, 64_100_000, 63_800_000, 63_300_000, 62_800_000, 62_300_000, 61_900_000, 61_500_000},
	// this year: depreciating a little each month
	{61_100_000, 60_600_000, 60_200_000, 59_800_000, 59_300_000, 58_700_000, 58_200_000, 57_800_000, 57_600_000, 57_300_000, 56_800_000, 56_400_000},
}

// demoMotorbike is the Motorbike's value, shedding roughly 10% a year.
var demoMotorbike = demoGrid{
	// 6 years ago: depreciating a little each month
	{14_000_000, 13_900_000, 13_800_000, 13_700_000, 13_550_000, 13_450_000, 13_350_000, 13_200_000, 13_100_000, 13_000_000, 12_900_000, 12_800_000},
	// 5 years ago: depreciating a little each month
	{12_650_000, 12_550_000, 12_450_000, 12_350_000, 12_250_000, 12_150_000, 12_050_000, 11_950_000, 11_800_000, 11_700_000, 11_600_000, 11_500_000},
	// 4 years ago: depreciating a little each month
	{11_400_000, 11_300_000, 11_200_000, 11_100_000, 11_050_000, 10_950_000, 10_800_000, 10_750_000, 10_650_000, 10_550_000, 10_450_000, 10_400_000},
	// 3 years ago: depreciating a little each month
	{10_300_000, 10_200_000, 10_150_000, 10_050_000, 9_950_000, 9_900_000, 9_800_000, 9_700_000, 9_600_000, 9_550_000, 9_450_000, 9_350_000},
	// 2 years ago: depreciating a little each month
	{9_250_000, 9_200_000, 9_100_000, 9_050_000, 8_950_000, 8_900_000, 8_800_000, 8_700_000, 8_650_000, 8_600_000, 8_500_000, 8_450_000},
	// 1 year ago: depreciating a little each month
	{8_400_000, 8_350_000, 8_250_000, 8_200_000, 8_150_000, 8_050_000, 8_000_000, 7_950_000, 7_850_000, 7_800_000, 7_700_000, 7_650_000},
	// this year: depreciating a little each month
	{7_550_000, 7_500_000, 7_450_000, 7_400_000, 7_350_000, 7_250_000, 7_200_000, 7_150_000, 7_100_000, 7_050_000, 6_950_000, 6_900_000},
}

// ---- Stocks, funds and gold ------------------------------------------------

// demoBBCA is Bank Central Asia (BBCA, sole, Demo): an opening lot, top-ups
// when cash allowed and the dividend that follows each year.
var demoBBCA = demoMarket{
	prices: demoGrid{
		// 6 years ago: 6,425 in January, 7,300 in December
		{6_425, 6_550, 6_550, 6_750, 6_650, 6_675, 6_900, 6_950, 6_875, 7_025, 7_050, 7_300},
		// 5 years ago: 7,350 in January, 7,850 in December
		{7_350, 7_275, 7_450, 7_300, 7_375, 7_600, 7_475, 7_600, 7_825, 7_750, 7_875, 7_850},
		// 4 years ago: 7,925 in January, 8,600 in December; dips about 7% in September and October, then mends
		{7_925, 7_925, 7_950, 8_175, 8_025, 8_225, 8_300, 8_175, 7_850, 7_600, 8_450, 8_600},
		// 3 years ago: 8,550 in January, 9,450 in December
		{8_550, 8_600, 8_875, 9_025, 9_050, 8_950, 9_000, 9_225, 9_400, 9_450, 9_475, 9_450},
		// 2 years ago: 9,600 in January, 10,000 in December
		{9_600, 9_625, 9_700, 9_625, 9_750, 9_925, 9_950, 9_800, 10_150, 10_150, 10_025, 10_000},
		// 1 year ago: 10,075 in January, 9,650 in December; slides from February to 7,500 in April, back above 9,000 by August
		{10_075, 9_150, 8_200, 7_500, 7_900, 8_500, 8_850, 9_150, 9_350, 9_550, 9_700, 9_650},
		// this year: 9,800 in January, 10,400 in December
		{9_800, 9_750, 9_900, 10_050, 9_950, 10_100, 10_250, 10_150, 10_300, 10_250, 10_350, 10_400},
	},
	trades: []demoTrade{
		{at(6, time.January), 1_800}, // opening lot
		{at(5, time.September), 300},
		{at(3, time.March), 300},
		{at(1, time.April), 400}, // buying the dip
		{at(0, time.June), 200},
	},
	cash: []demoCashEvent{
		{at(5, time.January), 20, repo.TxnTypeFee, 15_000},
		{at(5, time.April), 15, repo.TxnTypeDividend, 95_000},
		{at(4, time.April), 15, repo.TxnTypeDividend, 120_000},
		{at(3, time.January), 20, repo.TxnTypeFee, 18_000},
		{at(3, time.April), 15, repo.TxnTypeDividend, 155_000},
		{at(2, time.April), 15, repo.TxnTypeDividend, 170_000},
		{at(1, time.April), 15, repo.TxnTypeDividend, 190_000},
		{at(1, time.December), 20, repo.TxnTypeFee, 20_000},
		{at(0, time.April), 15, repo.TxnTypeDividend, 215_000},
	},
}

// demoASII is Astra International (ASII, joint, Short-term Goals): an opening
// lot, two later top-ups and a dividend each June (and an October interim some years).
var demoASII = demoMarket{
	prices: demoGrid{
		// 6 years ago: 4,350 in January, 4,675 in December
		{4_350, 4_375, 4_425, 4_475, 4_450, 4_425, 4_450, 4_500, 4_525, 4_700, 4_625, 4_675},
		// 5 years ago: 4,700 in January, 5,175 in December
		{4_700, 4_700, 4_850, 4_725, 4_950, 4_925, 4_950, 5_075, 5_025, 5_100, 5_050, 5_175},
		// 4 years ago: 5,150 in January, 5,450 in December; dips in September and October, then mends
		{5_150, 5_325, 5_250, 5_275, 5_325, 5_275, 5_375, 5_375, 4_800, 4_650, 5_600, 5_450},
		// 3 years ago: 5,625 in January, 5,775 in December
		{5_625, 5_500, 5_625, 5_725, 5_800, 5_675, 5_825, 5_700, 5_775, 5_925, 5_875, 5_775},
		// 2 years ago: 5,950 in January, 5,750 in December
		{5_950, 5_925, 5_875, 5_775, 5_750, 5_850, 5_875, 5_675, 5_800, 5_675, 5_775, 5_750},
		// 1 year ago: 5,650 in January, 5,350 in December; slides from February to 4,400 in April, back above 5,000 by July
		{5_650, 5_300, 4_800, 4_400, 4_600, 4_950, 5_150, 5_300, 5_300, 5_350, 5_400, 5_350},
		// this year: 5,575 in January, 5,475 in December
		{5_575, 5_500, 5_450, 5_650, 5_550, 5_600, 5_550, 5_500, 5_400, 5_425, 5_450, 5_475},
	},
	trades: []demoTrade{
		{at(6, time.January), 2_500}, // opening lot
		{at(4, time.June), 800},
		{at(1, time.May), 1_000}, // buying the dip
	},
	cash: []demoCashEvent{
		{at(5, time.June), 15, repo.TxnTypeDividend, 190_000},
		{at(4, time.June), 15, repo.TxnTypeDividend, 230_000},
		{at(4, time.October), 15, repo.TxnTypeDividend, 110_000},
		{at(3, time.June), 15, repo.TxnTypeDividend, 270_000},
		{at(2, time.June), 15, repo.TxnTypeDividend, 330_000},
		{at(2, time.October), 15, repo.TxnTypeDividend, 150_000},
		{at(1, time.June), 15, repo.TxnTypeDividend, 360_000},
		{at(0, time.June), 15, repo.TxnTypeDividend, 410_000},
	},
}

// demoEquityGrowthFund is the Equity Growth Fund (EGF01, sole, second member,
// Retirement): an opening lot, accumulating Buys, one partial profit-take Sell
// and a year-end Distribution some years.
var demoEquityGrowthFund = demoMarket{
	prices: demoGrid{
		// 6 years ago: 1,170 in January, 1,294 in December
		{1_170, 1_189, 1_190, 1_200, 1_229, 1_215, 1_223, 1_230, 1_241, 1_273, 1_285, 1_294},
		// 5 years ago: 1,285 in January, 1,403 in December
		{1_285, 1_312, 1_300, 1_324, 1_318, 1_341, 1_331, 1_342, 1_353, 1_374, 1_379, 1_403},
		// 4 years ago: 1,409 in January, 1,511 in December; dips in September and October, then mends
		{1_409, 1_413, 1_428, 1_418, 1_442, 1_456, 1_441, 1_469, 1_340, 1_295, 1_483, 1_511},
		// 3 years ago: 1,501 in January, 1,620 in December
		{1_501, 1_517, 1_514, 1_539, 1_558, 1_570, 1_559, 1_582, 1_609, 1_615, 1_624, 1_620},
		// 2 years ago: 1,643 in January, 1,729 in December
		{1_643, 1_660, 1_671, 1_646, 1_674, 1_663, 1_680, 1_695, 1_699, 1_723, 1_715, 1_729},
		// 1 year ago: 1,720 in January, 1,705 in December; slides from February to 1,360 in April, back above 1,600 by August
		{1_720, 1_620, 1_470, 1_360, 1_420, 1_520, 1_590, 1_650, 1_670, 1_685, 1_700, 1_705},
		// this year: 1,777 in January, 1,783 in December
		{1_777, 1_775, 1_784, 1_787, 1_764, 1_754, 1_769, 1_786, 1_790, 1_774, 1_777, 1_783},
	},
	trades: []demoTrade{
		{at(6, time.January), 12_000}, // opening lot
		{at(5, time.October), 3_000},
		{at(3, time.April), 3_500},
		{at(2, time.October), 3_000},
		{at(1, time.May), 5_000},       // buying the dip
		{at(0, time.February), -2_000}, // profit-take
	},
	cash: []demoCashEvent{
		{at(4, time.December), 15, repo.TxnTypeDistribution, 220_000},
		{at(2, time.December), 15, repo.TxnTypeDistribution, 380_000},
		{at(1, time.December), 15, repo.TxnTypeDistribution, 300_000},
	},
}

// demoMoneyMarketFund is the Money Market Fund (MMF01, joint): the household's
// parking spot for THR money, so most Buys land in April, with a Distribution
// in June or December some years.
var demoMoneyMarketFund = demoMarket{
	prices: demoGrid{
		// 6 years ago: 1,000 in January, 1,046 in December; NAV creeps up a few rupiah each month
		{1_000, 1_004, 1_008, 1_012, 1_017, 1_021, 1_025, 1_028, 1_033, 1_037, 1_041, 1_046},
		// 5 years ago: 1,050 in January, 1,096 in December
		{1_050, 1_054, 1_059, 1_062, 1_067, 1_072, 1_077, 1_081, 1_084, 1_088, 1_093, 1_096},
		// 4 years ago: 1,100 in January, 1,143 in December
		{1_100, 1_103, 1_106, 1_111, 1_115, 1_119, 1_123, 1_128, 1_131, 1_135, 1_139, 1_143},
		// 3 years ago: 1,147 in January, 1,190 in December
		{1_147, 1_151, 1_156, 1_160, 1_164, 1_167, 1_170, 1_174, 1_178, 1_181, 1_186, 1_190},
		// 2 years ago: 1,193 in January, 1,237 in December
		{1_193, 1_197, 1_201, 1_205, 1_209, 1_213, 1_218, 1_221, 1_225, 1_229, 1_233, 1_237},
		// 1 year ago: 1,242 in January, 1,286 in December
		{1_242, 1_246, 1_249, 1_254, 1_257, 1_261, 1_264, 1_269, 1_273, 1_278, 1_283, 1_286},
		// this year: 1,289 in January, 1,336 in December
		{1_289, 1_294, 1_298, 1_302, 1_307, 1_310, 1_315, 1_319, 1_323, 1_328, 1_332, 1_336},
	},
	trades: []demoTrade{
		{at(6, time.January), 10_000}, // opening lot
		{at(5, time.April), 5_000},    // THR
		{at(4, time.April), 6_000},    // THR
		{at(3, time.May), 5_000},
		{at(2, time.December), 4_000},
		{at(0, time.April), 3_000}, // THR
	},
	cash: []demoCashEvent{
		{at(5, time.June), 15, repo.TxnTypeDistribution, 60_000},
		{at(4, time.December), 15, repo.TxnTypeDistribution, 90_000},
		{at(2, time.June), 15, repo.TxnTypeDistribution, 140_000},
		{at(1, time.December), 15, repo.TxnTypeDistribution, 160_000},
		{at(0, time.June), 15, repo.TxnTypeDistribution, 185_000},
	},
}

// demoGoldBar is the Antam Gold Bar (sole, Retirement), in grams: an opening 4 g
// and a gram or two when the price allowed, plus a small storage fee each
// January some years.
var demoGoldBar = demoMarket{
	prices: demoGrid{
		// 6 years ago: 925,000 in January, 975,000 in December
		{925_000, 935_000, 920_000, 940_000, 955_000, 940_000, 955_000, 960_000, 965_000, 975_000, 985_000, 975_000},
		// 5 years ago: 990,000 in January, 1,055,000 in December
		{990_000, 985_000, 1_000_000, 1_010_000, 1_005_000, 1_015_000, 1_015_000, 1_025_000, 1_045_000, 1_050_000, 1_050_000, 1_055_000},
		// 4 years ago: 1,070,000 in January, 1,170,000 in December
		{1_070_000, 1_075_000, 1_090_000, 1_085_000, 1_095_000, 1_120_000, 1_115_000, 1_135_000, 1_150_000, 1_150_000, 1_145_000, 1_170_000},
		// 3 years ago: 1,170,000 in January, 1,330,000 in December
		{1_170_000, 1_180_000, 1_190_000, 1_225_000, 1_230_000, 1_255_000, 1_265_000, 1_265_000, 1_280_000, 1_290_000, 1_315_000, 1_330_000},
		// 2 years ago: 1,315,000 in January, 1,510,000 in December
		{1_315_000, 1_350_000, 1_380_000, 1_390_000, 1_385_000, 1_415_000, 1_415_000, 1_445_000, 1_445_000, 1_465_000, 1_505_000, 1_510_000},
		// 1 year ago: 1,515,000 in January, 1,655,000 in December
		{1_515_000, 1_535_000, 1_550_000, 1_565_000, 1_555_000, 1_560_000, 1_605_000, 1_620_000, 1_625_000, 1_645_000, 1_620_000, 1_655_000},
		// this year: 1,675,000 in January, 1,890,000 in December
		{1_675_000, 1_695_000, 1_715_000, 1_735_000, 1_755_000, 1_750_000, 1_805_000, 1_790_000, 1_815_000, 1_855_000, 1_860_000, 1_890_000},
	},
	trades: []demoTrade{
		{at(6, time.January), 4}, // opening lot
		{at(5, time.July), 1},
		{at(4, time.December), 2},
		{at(2, time.January), 1},
		{at(1, time.November), 2},
	},
	cash: []demoCashEvent{
		{at(5, time.January), 20, repo.TxnTypeFee, 8_000},
		{at(3, time.January), 20, repo.TxnTypeFee, 10_000},
		{at(1, time.January), 20, repo.TxnTypeFee, 12_000},
	},
}

// demoDigitalGold is Digital Gold Savings (joint), in grams: started four years
// ago in March and topped up twice. Zero price = not held yet.
var demoDigitalGold = demoMarket{
	prices: demoGrid{
		// 6 years ago: not held yet
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 5 years ago: not held yet
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 4 years ago: not held until March; then tracks the bar price, a touch under it
		{0, 0, 1_079_000, 1_069_000, 1_091_000, 1_109_000, 1_101_000, 1_129_000, 1_143_000, 1_145_000, 1_137_000, 1_155_000},
		// 3 years ago: tracks the bar price, a touch under it
		{1_160_000, 1_174_000, 1_181_000, 1_219_000, 1_218_000, 1_238_000, 1_255_000, 1_260_000, 1_277_000, 1_277_000, 1_301_000, 1_312_000},
		// 2 years ago: tracks the bar price, a touch under it
		{1_302_000, 1_332_000, 1_360_000, 1_373_000, 1_370_000, 1_412_000, 1_401_000, 1_436_000, 1_438_000, 1_456_000, 1_484_000, 1_498_000},
		// 1 year ago: tracks the bar price, a touch under it
		{1_493_000, 1_523_000, 1_539_000, 1_548_000, 1_539_000, 1_552_000, 1_592_000, 1_606_000, 1_621_000, 1_627_000, 1_600_000, 1_644_000},
		// this year: tracks the bar price, a touch under it
		{1_662_000, 1_684_000, 1_709_000, 1_731_000, 1_748_000, 1_724_000, 1_792_000, 1_766_000, 1_799_000, 1_829_000, 1_855_000, 1_884_000},
	},
	trades: []demoTrade{
		{at(4, time.March), 2}, // first purchase
		{at(3, time.October), 1},
		{at(1, time.June), 1},
	},
}

// ---- Bonds and time deposits -----------------------------------------------

// demoORIFace is the ORI024 face value, held at par so its snapshot is always
// 10M with zero accrued interest.
const demoORIFace = 10_000_000

// demoORI is the ORI024 retail bond (sole, Demo): a government-primary bond
// placed "2 years ago" in April out of the THR for a three-year term, paying
// out a monthly coupon after the 10% final tax (10M x 6.25% x 0.9 / 12).
var demoORI = demoBond{
	placed:  at(2, time.April),
	matures: at(-1, time.April),
	coupons: demoGrid{
		// 6 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 5 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 4 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 3 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 2 years ago
		{0, 0, 0, 0, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875},
		// 1 year ago
		{46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875},
		// this year
		{46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875, 46_875},
	},
}

// demoAstraBond is the Astra Sedaya corporate bond (joint, Short-term Goals): bought
// on the secondary market "3 years ago" in March at a slight premium, coupon
// paid semi-annually in June and December after tax (8M x 7.5% / 2 x 0.9). Its
// accrued interest builds 50k a month and resets at each coupon.
var demoAstraBond = demoBond{
	placed:  at(3, time.March),
	matures: at(-2, time.June),
	value: demoGrid{
		// 6 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 5 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 4 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 3 years ago
		{0, 0, 8_240_000, 8_290_000, 8_319_000, 8_100_000, 8_179_000, 8_260_000, 8_329_000, 8_381_000, 8_391_000, 8_140_000},
		// 2 years ago
		{8_190_000, 8_259_000, 8_320_000, 8_379_000, 8_431_000, 8_180_000, 8_229_000, 8_261_000, 8_269_000, 8_300_000, 8_370_000, 8_090_000},
		// 1 year ago
		{8_160_000, 8_180_000, 8_250_000, 8_320_000, 8_370_000, 8_150_000, 8_199_000, 8_260_000, 8_280_000, 8_330_000, 8_410_000, 8_120_000},
		// this year
		{8_171_000, 8_231_000, 8_280_000, 8_351_000, 8_400_000, 8_170_000, 8_220_000, 8_280_000, 8_301_000, 8_309_000, 8_341_000, 8_050_000},
	},
	accrued: demoGrid{
		// 6 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 5 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 4 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 3 years ago
		{0, 0, 150_000, 200_000, 249_000, 0, 49_000, 100_000, 149_000, 201_000, 251_000, 0},
		// 2 years ago
		{50_000, 99_000, 150_000, 199_000, 251_000, 0, 49_000, 101_000, 149_000, 200_000, 250_000, 0},
		// 1 year ago
		{50_000, 100_000, 150_000, 200_000, 250_000, 0, 49_000, 100_000, 150_000, 200_000, 250_000, 0},
		// this year
		{51_000, 101_000, 150_000, 201_000, 250_000, 0, 50_000, 100_000, 151_000, 199_000, 251_000, 0},
	},
	coupons: demoGrid{
		// 6 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 5 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 4 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 3 years ago
		{0, 0, 0, 0, 0, 270_000, 0, 0, 0, 0, 0, 270_000},
		// 2 years ago
		{0, 0, 0, 0, 0, 270_000, 0, 0, 0, 0, 0, 270_000},
		// 1 year ago
		{0, 0, 0, 0, 0, 270_000, 0, 0, 0, 0, 0, 270_000},
		// this year
		{0, 0, 0, 0, 0, 270_000, 0, 0, 0, 0, 0, 270_000},
	},
}

// demoMandiriTD is the Mandiri Time Deposit (sole, Education): 20M at 5.5%,
// auto-renew with interest, placed "2 years ago" in September out of Joint
// Savings for a 36-month term. Value = principal + accrued interest.
var demoMandiriTD = demoDeposit{
	placed:     at(2, time.September),
	termMonths: 36,
	value: demoGrid{
		// 6 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 5 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 4 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 3 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 2 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 20_052_000, 20_143_667, 20_235_334, 20_327_168},
		// 1 year ago
		{20_418_502, 20_510_169, 20_601_336, 20_693_003, 20_784_337, 20_875_671, 20_967_505, 21_058_839, 21_150_506, 21_242_173, 21_334_007, 21_425_674},
		// this year
		{21_517_341, 21_609_175, 21_700_842, 21_792_009, 21_883_843, 21_975_510, 22_066_677, 22_158_344, 22_250_344, 22_342_344, 22_433_511, 22_524_845},
	},
}

// demoBCATD is the BCA Time Deposit (joint), USD 1,500 at 2.5%, no rollover,
// placed "1 year ago" in March out of USD Savings for a 24-month term. In cents.
var demoBCATD = demoDeposit{
	placed:     at(1, time.March),
	termMonths: 24,
	value: demoGrid{
		// 6 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 5 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 4 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 3 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 2 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 1 year ago
		{0, 0, 150_220, 150_637, 151_054, 151_471, 151_888, 152_305, 152_722, 153_139, 153_556, 153_973},
		// this year
		{154_390, 154_807, 155_224, 155_641, 156_058, 156_475, 156_892, 157_309, 157_726, 158_143, 158_560, 158_977},
	},
}

// ---- Not updated this month ------------------------------------------------

// demoNotUpdated lists positions whose latest snapshot is older than the current
// month, by display name: the number of trailing months with no snapshot. The
// vehicles are revalued only now and then, the digital gold app is not opened every
// month and the friend repays when he can, so the dashboard's "not updated this
// month" list shows them. A carried-forward value cannot disturb the derived living
// expenses: a vehicle's or an investment's value change cancels out of the identity
// (INV-FINANCE-06/08), and the friend's repayments sit in the older months.
var demoNotUpdated = map[string]int{
	"Family Car":             2,
	"Motorbike":              3,
	"Digital Gold Savings":   1,
	"Money Lent to a Friend": 2,
}

// ---- Closed positions ------------------------------------------------------

// demoOldHatchback is the Old Hatchback (sole, Demo): the household's first car, sold
// "3 years ago" in August to a neighbour. It is recorded until July; the sale is
// the lifecycle flip to "sold", which writes the zero close snapshot, and the
// proceeds simply arrive in the bank (Everyday Checking). Zero = sold.
var demoOldHatchback = demoClosedAsset{
	closing: demoClosing{at(3, time.August), 18, "Sold to a neighbour"},
	value: demoGrid{
		// 6 years ago: depreciating a little each month
		{52_000_000, 51_500_000, 51_100_000, 50_800_000, 50_400_000, 50_100_000, 49_700_000, 49_300_000, 48_900_000, 48_500_000, 48_100_000, 47_700_000},
		// 5 years ago: depreciating a little each month
		{47_400_000, 47_100_000, 46_800_000, 46_400_000, 46_100_000, 45_800_000, 45_400_000, 45_000_000, 44_700_000, 44_400_000, 43_900_000, 43_600_000},
		// 4 years ago: depreciating a little each month
		{43_300_000, 43_000_000, 42_700_000, 42_400_000, 42_000_000, 41_700_000, 41_400_000, 41_200_000, 40_800_000, 40_500_000, 40_200_000, 39_800_000},
		// 3 years ago: depreciating a little each month
		{39_500_000, 39_100_000, 38_900_000, 38_600_000, 38_200_000, 37_900_000, 37_500_000, 0, 0, 0, 0, 0},
		// 2 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 1 year ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// this year
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	},
}

// demoTLKM is Telkom Indonesia (TLKM, sole, second member): an opening lot, two
// top-ups and a June dividend each year, then fully sold "2 years ago" in June
// after a long flat run. The terminal Sell is written by the lifecycle flip at that
// month's price, so the holding closes at zero.
var demoTLKM = demoMarket{
	prices: demoGrid{
		// 6 years ago: 3,280 in January, 3,500 in December
		{3_280, 3_320, 3_350, 3_380, 3_310, 3_420, 3_430, 3_360, 3_420, 3_400, 3_430, 3_500},
		// 5 years ago: 3,460 in January, 3,570 in December
		{3_460, 3_460, 3_490, 3_540, 3_520, 3_560, 3_570, 3_500, 3_580, 3_500, 3_560, 3_570},
		// 4 years ago: 3,570 in January, 3,650 in December
		{3_570, 3_530, 3_540, 3_550, 3_590, 3_580, 3_660, 3_680, 3_630, 3_630, 3_680, 3_650},
		// 3 years ago: 3,740 in January, 3,620 in December
		{3_740, 3_700, 3_750, 3_680, 3_730, 3_640, 3_640, 3_650, 3_720, 3_670, 3_690, 3_620},
		// 2 years ago: 3,640 in January, 3,670 in December; sold in June
		{3_640, 3_650, 3_710, 3_650, 3_690, 3_680, 3_620, 3_600, 3_670, 3_650, 3_580, 3_670},
		// 1 year ago: 3,620 in January, 3,580 in December
		{3_620, 3_570, 3_570, 3_560, 3_640, 3_640, 3_550, 3_550, 3_570, 3_550, 3_620, 3_580},
		// this year: 3,590 in January, 3,460 in December
		{3_590, 3_520, 3_590, 3_570, 3_550, 3_480, 3_490, 3_560, 3_510, 3_530, 3_530, 3_460},
	},
	trades: []demoTrade{
		{at(6, time.January), 3_000}, // opening lot
		{at(5, time.November), 500},
		{at(3, time.March), 500},
	},
	cash: []demoCashEvent{
		{at(5, time.June), 15, repo.TxnTypeDividend, 420_000},
		{at(4, time.June), 15, repo.TxnTypeDividend, 480_000},
		{at(3, time.June), 15, repo.TxnTypeDividend, 560_000},
		{at(2, time.June), 15, repo.TxnTypeDividend, 600_000},
	},
	closing: &demoClosing{at(2, time.June), 20, "Sold the whole holding"},
}

// demoBNIDeposit is the BNI Time Deposit (joint): 15M at 5.0%, auto-renew with
// interest, placed "5 years ago" in March out of Joint Savings for 36 months. It
// matured "2 years ago" in March with 2.25M of interest, and both legs rolled into
// demoBNIRenewal: a Maturity transaction (rolled_to_new), a successor holding
// 17.25M and the rollover link. Recorded up to February.
var demoBNIDeposit = demoMaturedDeposit{
	placed:     at(5, time.March),
	termMonths: 36,
	principal:  15_000_000,
	interest:   2_250_000,
	value: demoGrid{
		// 6 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 5 years ago
		{0, 0, 15_037_500, 15_099_700, 15_162_200, 15_225_100, 15_287_300, 15_349_800, 15_412_300, 15_474_800, 15_537_200, 15_599_400},
		// 4 years ago
		{15_661_900, 15_724_300, 15_786_700, 15_849_400, 15_911_600, 15_974_300, 16_037_000, 16_099_500, 16_162_400, 16_225_300, 16_287_700, 16_350_200},
		// 3 years ago
		{16_412_600, 16_474_800, 16_537_200, 16_599_900, 16_662_800, 16_725_200, 16_787_900, 16_850_100, 16_912_300, 16_974_800, 17_037_500, 17_100_000},
		// 2 years ago
		{17_162_500, 17_225_000, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 1 year ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// this year
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	},
}

// demoBNIRenewal is the BNI Time Deposit (renewed): the successor of
// demoBNIDeposit, placed on its maturity date with the rolled principal and
// interest (17.25M) for another 36 months, still running.
var demoBNIRenewal = demoDeposit{
	placed:     at(2, time.March),
	termMonths: 36,
	value: demoGrid{
		// 6 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 5 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 4 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 3 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 2 years ago
		{0, 0, 17_293_125, 17_365_000, 17_436_875, 17_509_250, 17_581_425, 17_653_600, 17_725_475, 17_797_150, 17_868_625, 17_940_500},
		// 1 year ago
		{18_012_375, 18_084_550, 18_156_925, 18_228_400, 18_300_075, 18_372_250, 18_444_625, 18_516_800, 18_588_275, 18_660_150, 18_731_625, 18_804_000},
		// this year
		{18_875_875, 18_947_750, 19_019_625, 19_091_500, 19_163_675, 19_235_850, 19_307_325, 19_379_200, 19_451_375, 19_523_250, 19_595_425, 19_667_300},
	},
}

// demoDanamonDeposit is the Danamon Time Deposit (sole, second member): 10M at
// 5.25%, no rollover, placed "4 years ago" in June out of Joint Savings for 12
// months. It matured "3 years ago" in June and the 10M plus 525k of interest was
// paid out to the bank (settled through the Terminate dialog's Maturity). Recorded
// up to May.
var demoDanamonDeposit = demoMaturedDeposit{
	placed:     at(4, time.June),
	termMonths: 12,
	principal:  10_000_000,
	interest:   525_000,
	value: demoGrid{
		// 6 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 5 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 4 years ago
		{0, 0, 0, 0, 0, 10_026_250, 10_070_000, 10_113_750, 10_157_800, 10_201_350, 10_245_400, 10_289_450},
		// 3 years ago
		{10_333_200, 10_376_850, 10_420_600, 10_464_500, 10_508_050, 0, 0, 0, 0, 0, 0, 0},
		// 2 years ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// 1 year ago
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		// this year
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	},
}

// ---- Liabilities and receivables -------------------------------------------

// demoMortgage is the Home Mortgage balance: a 185M, 8.5%, 15-year loan that
// amortises on a fixed instalment, with two prepayments out of the THR.
var demoMortgage = demoGrid{
	// 6 years ago: 181.34M in January, 175.22M by December; a fixed instalment of about 1.8M, mostly interest at first
	{181_344_000, 180_806_000, 180_265_000, 179_720_000, 179_172_000, 178_619_000, 178_062_000, 177_502_000, 176_937_000, 176_369_000, 175_797_000, 175_220_000},
	// 5 years ago: 174.64M in January, 167.97M by December
	{174_639_000, 174_055_000, 173_466_000, 172_873_000, 172_275_000, 171_674_000, 171_068_000, 170_458_000, 169_844_000, 169_225_000, 168_602_000, 167_975_000},
	// 4 years ago: 167.34M in January, 155.86M by December; 4M prepaid in April out of the THR
	{167_343_000, 166_706_000, 166_065_000, 161_420_000, 160_741_000, 160_058_000, 159_370_000, 158_677_000, 157_979_000, 157_277_000, 156_569_000, 155_856_000},
	// 3 years ago: 155.14M in January, 146.90M by December
	{155_138_000, 154_416_000, 153_688_000, 152_954_000, 152_216_000, 151_473_000, 150_724_000, 149_970_000, 149_210_000, 148_445_000, 147_675_000, 146_899_000},
	// 2 years ago: 146.12M in January, 130.80M by December; 6M prepaid in April out of the THR
	{146_118_000, 145_331_000, 144_539_000, 137_741_000, 136_895_000, 136_043_000, 135_185_000, 134_320_000, 133_450_000, 132_574_000, 131_691_000, 130_802_000},
	// 1 year ago: 129.91M in January, 119.63M by December
	{129_907_000, 129_005_000, 128_097_000, 127_183_000, 126_262_000, 125_334_000, 124_400_000, 123_460_000, 122_512_000, 121_558_000, 120_598_000, 119_630_000},
	// this year: 118.66M in January, 107.47M by December
	{118_656_000, 117_675_000, 116_686_000, 115_691_000, 114_689_000, 113_679_000, 112_663_000, 111_639_000, 110_608_000, 109_570_000, 108_524_000, 107_471_000},
}

// demoParentsLoan is the Loan from Parents balance, repaid in sporadic chunks.
var demoParentsLoan = demoGrid{
	// 6 years ago: 9.50M in January, 8.00M by December; repaid in uneven chunks, flat in between
	{9_500_000, 9_500_000, 9_500_000, 9_000_000, 9_000_000, 9_000_000, 9_000_000, 8_750_000, 8_750_000, 8_000_000, 8_000_000, 8_000_000},
	// 5 years ago: 8.00M in January, 6.25M by December
	{8_000_000, 8_000_000, 7_500_000, 7_500_000, 7_500_000, 7_500_000, 7_500_000, 7_250_000, 7_250_000, 7_250_000, 7_250_000, 6_250_000},
	// 4 years ago: 6.25M in January, 5.50M by December
	{6_250_000, 6_250_000, 6_250_000, 5_750_000, 5_750_000, 5_750_000, 5_750_000, 5_750_000, 5_750_000, 5_750_000, 5_500_000, 5_500_000},
	// 3 years ago: 5.50M in January, 4.25M by December
	{5_500_000, 5_500_000, 5_500_000, 5_500_000, 5_500_000, 4_750_000, 4_750_000, 4_750_000, 4_250_000, 4_250_000, 4_250_000, 4_250_000},
	// 2 years ago: 4.25M in January, 3.50M by December
	{4_250_000, 4_250_000, 4_250_000, 4_250_000, 4_000_000, 4_000_000, 4_000_000, 4_000_000, 4_000_000, 3_500_000, 3_500_000, 3_500_000},
	// 1 year ago: 3.50M in January, 2.25M by December
	{3_500_000, 3_500_000, 3_500_000, 2_500_000, 2_500_000, 2_500_000, 2_500_000, 2_500_000, 2_500_000, 2_250_000, 2_250_000, 2_250_000},
	// this year: 2.25M in January, 1.50M by December
	{2_250_000, 2_250_000, 2_250_000, 1_750_000, 1_750_000, 1_750_000, 1_750_000, 1_750_000, 1_500_000, 1_500_000, 1_500_000, 1_500_000},
}

// demoFriendLoan is the Money Lent to a Friend balance, repaid in small
// instalments.
var demoFriendLoan = demoGrid{
	// 6 years ago: 3.20M in January, 2.85M by December; repaid in small instalments, flat in between
	{3_200_000, 3_200_000, 3_100_000, 3_100_000, 3_100_000, 3_100_000, 2_950_000, 2_950_000, 2_950_000, 2_950_000, 2_950_000, 2_850_000},
	// 5 years ago: 2.85M in January, 2.55M by December
	{2_850_000, 2_850_000, 2_850_000, 2_650_000, 2_650_000, 2_650_000, 2_650_000, 2_650_000, 2_650_000, 2_650_000, 2_550_000, 2_550_000},
	// 4 years ago: 2.55M in January, 2.30M by December
	{2_550_000, 2_550_000, 2_550_000, 2_550_000, 2_550_000, 2_400_000, 2_400_000, 2_400_000, 2_400_000, 2_300_000, 2_300_000, 2_300_000},
	// 3 years ago: 2.30M in January, 1.95M by December
	{2_300_000, 2_300_000, 2_300_000, 2_300_000, 2_050_000, 2_050_000, 2_050_000, 2_050_000, 2_050_000, 2_050_000, 2_050_000, 1_950_000},
	// 2 years ago: 1.95M in January, 1.75M by December
	{1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_950_000, 1_750_000, 1_750_000, 1_750_000, 1_750_000, 1_750_000},
	// 1 year ago: 1.65M in January, 1.50M by December
	{1_650_000, 1_650_000, 1_650_000, 1_650_000, 1_650_000, 1_650_000, 1_650_000, 1_650_000, 1_500_000, 1_500_000, 1_500_000, 1_500_000},
	// this year: 1.50M in January, 1.25M by December
	{1_500_000, 1_500_000, 1_400_000, 1_400_000, 1_400_000, 1_400_000, 1_400_000, 1_250_000, 1_250_000, 1_250_000, 1_250_000, 1_250_000},
}

// demoFreelanceInvoice is the Freelance Invoice Pending balance, in cents.
var demoFreelanceInvoice = demoGrid{
	// 6 years ago: $540 in January, $515 by December; the client settles in small USD instalments
	{54_000, 54_000, 54_000, 54_000, 54_000, 51_500, 51_500, 51_500, 51_500, 51_500, 51_500, 51_500},
	// 5 years ago: $475 in January, $445 by December
	{47_500, 47_500, 47_500, 47_500, 47_500, 47_500, 44_500, 44_500, 44_500, 44_500, 44_500, 44_500},
	// 4 years ago: $445 in January, $365 by December
	{44_500, 44_500, 39_500, 39_500, 39_500, 39_500, 39_500, 39_500, 39_500, 36_500, 36_500, 36_500},
	// 3 years ago: $365 in January, $325 by December
	{36_500, 36_500, 36_500, 36_500, 36_500, 32_500, 32_500, 32_500, 32_500, 32_500, 32_500, 32_500},
	// 2 years ago: $325 in January, $260 by December
	{32_500, 32_500, 29_000, 29_000, 29_000, 29_000, 29_000, 29_000, 29_000, 29_000, 26_000, 26_000},
	// 1 year ago: $260 in January, $215 by December
	{26_000, 26_000, 26_000, 26_000, 26_000, 26_000, 21_500, 21_500, 21_500, 21_500, 21_500, 21_500},
	// this year: $215 in January, $160 by December
	{21_500, 21_500, 18_500, 18_500, 18_500, 18_500, 18_500, 18_500, 16_000, 16_000, 16_000, 16_000},
}

// ---- Rates -----------------------------------------------------------------

// demoUSDRate is the IDR per USD rate entered for each month.
var demoUSDRate = demoGrid{
	// 6 years ago: 14,625 in January, 14,855 by December
	{14_625, 14_670, 14_680, 14_735, 14_710, 14_765, 14_740, 14_820, 14_810, 14_810, 14_855, 14_855},
	// 5 years ago: 14,875 in January, 15,175 by December
	{14_875, 14_925, 14_915, 14_970, 14_980, 15_065, 15_110, 15_120, 15_120, 15_195, 15_230, 15_175},
	// 4 years ago: 15,260 in January, 15,495 by December
	{15_260, 15_255, 15_325, 15_305, 15_315, 15_315, 15_395, 15_405, 15_430, 15_410, 15_420, 15_495},
	// 3 years ago: 15,505 in January, 15,740 by December
	{15_505, 15_545, 15_525, 15_555, 15_560, 15_580, 15_660, 15_610, 15_705, 15_655, 15_710, 15_740},
	// 2 years ago: 15,745 in January, 16,105 by December
	{15_745, 15_815, 15_770, 15_860, 15_840, 15_930, 15_950, 15_970, 15_990, 16_025, 16_035, 16_105},
	// 1 year ago: 16,115 in January, 16,260 by December; a spike to 16,640 in May
	{16_115, 16_120, 16_270, 16_520, 16_640, 16_580, 16_450, 16_380, 16_280, 16_265, 16_255, 16_260},
	// this year: 16,285 in January, 16,435 by December
	{16_285, 16_355, 16_370, 16_340, 16_380, 16_395, 16_375, 16_410, 16_385, 16_405, 16_395, 16_435},
}

// demoInflation is the monthly annual-CPI figure, in hundredths of a percent;
// zero = not entered. Only the last three years are entered, so the oldest
// months exercise the assumed-inflation fallback.
var demoInflation = demoGrid{
	// 6 years ago: not entered (the assumed-inflation fallback covers it)
	{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	// 5 years ago: not entered (the assumed-inflation fallback covers it)
	{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	// 4 years ago: not entered (the assumed-inflation fallback covers it)
	{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	// 3 years ago: not entered (the assumed-inflation fallback covers it)
	{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	// 2 years ago: 3.2% easing down to 2.6%
	{308, 310, 293, 288, 287, 284, 273, 277, 273, 268, 270, 257},
	// 1 year ago: around 2.6%, firming to 2.8% by December
	{260, 266, 250, 251, 260, 259, 247, 265, 274, 283, 288, 288},
	// this year: just under 3%, ending at 2.8%
	{294, 301, 300, 301, 310, 307, 310, 303, 295, 286, 285, 287},
}
