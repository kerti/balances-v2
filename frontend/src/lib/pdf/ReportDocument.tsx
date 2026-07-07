import { Document, Page, StyleSheet, Text, View } from "@react-pdf/renderer";
import type { TFunction } from "i18next";
import type { HouseholdMember } from "@/api/types";
import type { Me } from "@/hooks/useSession";
import { formatCurrency, formatNumber, formatYearMonth } from "@/lib/format";
import { preferredName } from "@/lib/names";
import type { ReportPdfData } from "@/lib/pdf/reportPdfData";
import { LineChart } from "@/lib/pdf/charts/LineChart";
import { Wordmark } from "@/lib/pdf/Wordmark";

// Rendered outside the app's React tree (react-pdf uses its own reconciler —
// see ADR-0044), so no hooks: `t` is a fixed translator (i18n.getFixedT) the
// caller derives from the live app locale, not a live useTranslation() binding.
type Props = {
  data: ReportPdfData;
  t: TFunction;
  members: HouseholdMember[] | undefined;
  me: Me | null | undefined;
};

// Mirrors DashboardScreen.tsx's personLabel — kept as a separate copy rather
// than shared, matching ADR-0044's decision not to refactor the live
// dashboard for this addition.
function personLabel(
  t: TFunction,
  key: string,
  members: HouseholdMember[] | undefined,
  me: Me | null | undefined,
): string {
  if (key === "joint") return t("byPerson.joint");
  const m = (members ?? []).find((x) => x.id === key);
  if (!m) return t("byPerson.unknown");
  return me && m.id === me.id ? `${preferredName(m)}${t("byPerson.youSuffix")}` : preferredName(m);
}

const styles = StyleSheet.create({
  page: { padding: 32, fontSize: 10, color: "#0F172A", fontFamily: "Helvetica" },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    marginBottom: 16,
  },
  month: { fontSize: 12, color: "#64748B" },
  headlineTotal: { fontSize: 24, fontWeight: 700, marginBottom: 2 },
  headlineSecondary: { fontSize: 11, color: "#64748B", marginBottom: 12 },
  sectionTitle: { fontSize: 11, fontWeight: 700, marginTop: 16, marginBottom: 6 },
  row: { flexDirection: "row", justifyContent: "space-between", paddingVertical: 2 },
  rowLabel: { color: "#334155" },
  rowValue: { fontWeight: 700 },
  muted: { color: "#64748B" },
});

export function ReportDocument({ data, t, members, me }: Props) {
  return (
    <Document>
      <Page size="A4" style={styles.page}>
        <View style={styles.header}>
          <Wordmark />
          <Text style={styles.month}>{formatYearMonth(data.yearMonth)}</Text>
        </View>

        <Text style={styles.headlineTotal}>
          {formatCurrency(data.headline.total, data.currency)}
        </Text>
        {data.headline.secondary && (
          <Text style={styles.headlineSecondary}>
            {formatCurrency(
              String(data.headline.secondary.amount),
              data.headline.secondary.currency,
            )}
          </Text>
        )}

        <LineChart series={data.series} />

        <Text style={styles.sectionTitle}>{t("breakdown.title")}</Text>
        {data.groupBreakdown.map((row) => (
          <View key={row.labelKey} style={styles.row}>
            <Text style={styles.rowLabel}>{t(`breakdown.${row.labelKey}`)}</Text>
            <Text style={styles.rowValue}>{formatCurrency(String(row.value), data.currency)}</Text>
          </View>
        ))}

        {data.fxRatesUsed.length > 0 && (
          <>
            <Text style={styles.sectionTitle}>{t("fxThisMonth.title")}</Text>
            {data.fxRatesUsed.map((r) => (
              <Text key={r.currency} style={styles.muted}>
                {t("fxThisMonth.line", {
                  base: r.currency,
                  rate: formatNumber(r.rate),
                  quote: data.currency,
                })}
              </Text>
            ))}
          </>
        )}

        <Text style={styles.sectionTitle}>{t("thisMonth.title")}</Text>
        {data.incomeStatement === null ? (
          <Text style={styles.muted}>{t("thisMonth.baseline")}</Text>
        ) : (
          <>
            <View style={styles.row}>
              <Text style={styles.rowLabel}>{t("statement.earned")}</Text>
              <Text>{formatCurrency(String(data.incomeStatement.earned), data.currency)}</Text>
            </View>
            <View style={styles.row}>
              <Text style={styles.rowLabel}>{t("statement.investmentReturn")}</Text>
              <Text>
                {formatCurrency(String(data.incomeStatement.investmentReturn), data.currency)}
              </Text>
            </View>
            {data.incomeStatement.assetValueChange !== 0 && (
              <View style={styles.row}>
                <Text style={styles.rowLabel}>{t("statement.assetValueChange")}</Text>
                <Text style={styles.muted}>
                  {formatCurrency(String(data.incomeStatement.assetValueChange), data.currency)}
                </Text>
              </View>
            )}
            <View style={styles.row}>
              <Text style={styles.rowLabel}>{t("statement.livingExpenses")}</Text>
              <Text style={styles.muted}>
                {formatCurrency(String(data.incomeStatement.livingExpenses), data.currency)}
              </Text>
            </View>
            <View style={styles.row}>
              <Text style={styles.rowLabel}>{t("statement.nwChange")}</Text>
              <Text style={styles.rowValue}>
                {formatCurrency(String(data.incomeStatement.netWorthChange), data.currency)}
              </Text>
            </View>
          </>
        )}

        <Text style={styles.sectionTitle}>{t("byPerson.title")}</Text>
        {data.byPerson.map((p) => (
          <View key={p.key} style={styles.row}>
            <Text style={styles.rowLabel}>{personLabel(t, p.key, members, me)}</Text>
            <Text>{formatCurrency(p.nw, data.currency)}</Text>
          </View>
        ))}
      </Page>
    </Document>
  );
}
