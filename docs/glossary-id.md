# Indonesian glossary

The canonical English ↔ Indonesian dictionary for balances-v2 UI copy. Translation work for issues
#5–#11 references this file; new terms expand it rather than getting decided inline. EN extraction
proceeds in parallel.

The English column is the source of truth and matches the domain terms in [CONTEXT.md](../CONTEXT.md).
The Indonesian column is what user-facing ID copy must use. The **Notes** column records the
rationale where the choice wasn't obvious and lists synonyms to actively avoid on the same screen.

When extending: add the row, follow the existing column conventions (Title Case for nouns, lower
case for action verbs the way they appear in buttons), and link the row from the PR description if
the choice is non-obvious.

## Position groups

| English | Indonesian | Notes |
| --- | --- | --- |
| Asset | Aset | _Avoid_: Aktiva (formal accounting register, reads stiff in household UI). |
| Liability | Liabilitas | _Avoid_: Kewajiban (legalistic; ambiguous with general obligations). Modern Indonesian accounting prefers Liabilitas. |
| Receivable | Piutang | Canonical Indonesian finance term. |
| Investment | Investasi | — |
| Income | Pemasukan | More colloquial than Pendapatan; the app targets non-technical household members. Pendapatan stays acceptable in formal report headings. |
| Expense (future) | Pengeluaran | Reserved for the future expense-tracking surface (not modelled today; cashflow is a residual). |

## Subtypes

| English | Indonesian | Notes |
| --- | --- | --- |
| Bank Account | Rekening Bank | Short form "Rekening" acceptable in tight columns. |
| Property | Properti | — |
| Vehicle | Kendaraan | — |
| Personal Liability | Liabilitas Pribadi | Owed informally to family/friends. |
| Institutional Liability | Liabilitas Institusional | Loan from a bank/lender. _Avoid_: Lembaga (ambiguous). |
| Stock | Saham | — |
| Mutual Fund | Reksa Dana | Canonical (two words). |
| Bond | Obligasi | **Deliberate divergence from CONTEXT.md's _Avoid_ list.** "Obligasi" is the standard Indonesian finance term and is the right ID translation. The CONTEXT avoid list applies to *English* code/UI (Bond is canonical there); it does not constrain the ID catalog. |
| Time Deposit | Deposito | Short form is the everyday Indonesian usage. Use "Deposito Berjangka" only where formality demands. _Avoid_: standalone "Simpanan" (overlaps with bank-account cash). |
| Gold | Emas | — |

## Ledger nouns

| English | Indonesian | Notes |
| --- | --- | --- |
| Snapshot | Snapshot | Keep the English loanword — it's the app's product noun and "Cuplikan" reads awkward in a finance context. |
| Transaction | Transaksi | — |
| Buy | Beli | Verb form for the Buy transaction type. |
| Sell | Jual | — |
| Coupon | Kupon | Bond interest payment. |
| Dividend | Dividen | — |
| Distribution | Distribusi | Mutual-fund payouts. |
| Fee | Biaya | _Avoid_: Fee (English loanword reads informal in totals). |
| Maturity | Jatuh Tempo | Two words; canonical. Used as the transaction type ("Jatuh Tempo") and as the date label ("Tanggal Jatuh Tempo"). |
| Selling price | Harga jual | Gold (issue #19): the dealer's selling price — what you pay to buy gold. The higher side of the spread. |
| Buyback price | Harga buyback | Gold (issue #19): what a dealer pays you when you sell — the lower side of the spread and the price gold snapshots mark at. "Buyback" is the everyday Indonesian usage (Antam/Pegadaian use it directly); _Avoid_: "Harga beli kembali" (correct but stiff). |

## Lifecycle

| English | Indonesian | Notes |
| --- | --- | --- |
| Active | Aktif | — |
| Terminated (generic) | Berakhir | Generic fallback when no group-specific verb fits. |
| Closed (bank account) | Ditutup | — |
| Reopened | Dibuka Kembali | — |
| Sold (vehicle / property / investment) | Terjual | — |
| Disposed | Dilepas | — |
| Matured (bond / time deposit) | Jatuh Tempo | Same surface form as the Maturity transaction; context disambiguates. |
| Paid off (liability) | Lunas | — |
| Forgiven (liability) | Dibebaskan | — |
| Written off (liability / receivable) | Dihapusbukukan | Canonical accounting verb. |
| Collected (receivable) | Tertagih | — |

## Money and accounting

| English | Indonesian | Notes |
| --- | --- | --- |
| Net Worth | Kekayaan Bersih | — |
| Balance | Saldo | — |
| Amount | Nominal | _Avoid mixing with_: Jumlah on the same screen — "Jumlah" is reserved for Quantity below to keep the two distinct. |
| Currency | Mata Uang | — |
| Reporting Currency | Mata Uang Pelaporan | — |
| Native Currency | Mata Uang Asal | The currency a position is denominated in. |
| FX Rate | Kurs | Canonical for foreign-exchange rate. |
| Interest Rate | Suku Bunga | — |
| Interest | Bunga | — |
| Appreciation | Apresiasi | Property revaluation up. _Avoid_: Kenaikan Nilai (reads vague). |
| Depreciation | Penyusutan | Vehicle / asset value loss. _Avoid_: Depresiasi (loanword, less common than Penyusutan in accounting). |
| Principal | Pokok | "Pokok pinjaman" (liability) / "Pokok deposito" (time deposit). |
| Face Value | Nilai Nominal | Bond face value. |
| Quantity | Jumlah | Reserved for counts (shares, units, grams). Don't reuse for monetary amount — that's Nominal. |
| Price per Unit | Harga per Unit | — |
| Accrued Interest | Bunga Berjalan | Canonical retail-bond term. _Avoid_: Bunga Akrual (technically correct but reads textbook-y). |
| Cost / Cost Basis | Modal | Money put into a position. Retail-friendly; "Biaya perolehan" is the accounting term but reads jargon-y for the household audience. _Avoid_: Harga Pokok (stockbroker-specific). |
| Unrealized P/L | Untung/rugi belum direalisasi | Gap between current value and cost basis. Long-form deliberate — abbreviation "L/R" (laba/rugi) reads too report-y for non-technical users. |
| Fee | Biaya | Transaction fee. Distinct from Modal (cost basis) by context. |

## Time and dates

| English | Indonesian | Notes |
| --- | --- | --- |
| Snapshot Date | Tanggal Snapshot | — |
| Maturity Date | Tanggal Jatuh Tempo | — |
| Placement Date | Tanggal Penempatan | Time-deposit start date. |
| Year-Month | Bulan | Single column header showing YYYY-MM. "Periode Bulan" if disambiguation needed. |
| Period | Periode | — |

## Auth and household

| English | Indonesian | Notes |
| --- | --- | --- |
| Household | Rumah Tangga | — |
| Member | Anggota | — |
| User | Pengguna | — |
| Owner | Pemilik | — |
| Joint (ownership) | Bersama | "Kepemilikan Bersama" in full. |
| Sole (ownership) | Tunggal | "Pemilik Tunggal" in full. _Avoid_: Pribadi (overloads with the "Personal" liability subtype). |
| Invite (noun) | Undangan | — |
| Invite (verb) | Undang | — |
| Sign in | Masuk | — |
| Sign out | Keluar | — |
| Erasure (whole-household hard delete, ADR-0040) | Hapus (rumah tangga) | Rendered as a plain verb phrase ("Hapus rumah tangga ini"), not a noun loanword — matches Restore's existing informal treatment ("Pulihkan"/"cadangan" have no dedicated glossary row either). _Avoid_: Penghapusan (correct but reads bureaucratic for a destructive-action button). |

## Risk and regularity

| English | Indonesian | Notes |
| --- | --- | --- |
| Low Risk | Risiko Rendah | — |
| Medium Risk | Risiko Sedang | — |
| High Risk | Risiko Tinggi | — |
| Routine | Rutin | Income regularity. |
| Incidental | Insidental | Income regularity. Loanword is well-established in Indonesian financial writing. |

## Income categories

| English | Indonesian | Notes |
| --- | --- | --- |
| Salary | Gaji | — |
| Business Income | Pendapatan Usaha | — |
| Rental Income | Pendapatan Sewa | — |
| Gift | Hadiah | — |
| Tax Refund | Pengembalian Pajak | — |
| Insurance Payout | Klaim Asuransi | — |
| Other | Lainnya | — |

## Actions and chrome

| English | Indonesian | Notes |
| --- | --- | --- |
| Create / Add | Tambah | Used on primary action buttons ("Tambah Aset", "Tambah Snapshot"). |
| Edit | Ubah | _Avoid_: Edit (English loanword) for buttons. |
| Delete | Hapus | — |
| Save | Simpan | — |
| Cancel | Batal | — |
| Confirm | Konfirmasi | — |
| Close | Tutup | — |
| Reopen | Buka Kembali | — |
| Filter | Filter | Loanword acceptable; "Saring" reads stiff. |
| Import | Impor | — |
| Export | Ekspor | — |
| Search | Cari | — |
| Settings | Pengaturan | — |
| Language | Bahasa | — |

## Errors

| English | Indonesian | Notes |
| --- | --- | --- |
| Something went wrong | Terjadi kesalahan | Generic fallback toast. |
| Not found | Tidak ditemukan | — |
| Failed to save | Gagal menyimpan | — |
| Failed to delete | Gagal menghapus | — |
| Failed to load | Gagal memuat | — |
