import { Document, Page, StyleSheet, Text, View } from "@react-pdf/renderer";
import type { TFunction } from "i18next";
import type { HouseholdMember } from "@/api/types";
import type { Me } from "@/hooks/useSession";
import { formatCurrency, formatNumber, formatYearMonth } from "@/lib/format";
import { preferredName } from "@/lib/names";
import type { CompositionSlice, ItemizedPosition, ReportPdfData } from "@/lib/pdf/reportPdfData";
import { LineChart } from "@/lib/pdf/charts/LineChart";
import { PieChart, PieChartLegend } from "@/lib/pdf/charts/PieChart";
import { TrendChart } from "@/lib/pdf/charts/TrendChart";
import {
  TREND_INVESTMENT_RETURN_COLOR,
  TREND_LIVING_EXPENSES_COLOR,
} from "@/lib/pdf/charts/lineChartMath";
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

// GROUP_ORDER matches groupBreakdown's fixed order; breakdownKey maps
// PositionDetail's singular `group` to the plural label keys the existing
// "Where it's held" section already uses (`breakdown.assets` etc.) — reused
// rather than duplicated (ADR-0045).
const GROUP_ORDER = [
  { group: "asset", breakdownKey: "assets" },
  { group: "investment", breakdownKey: "investments" },
  { group: "receivable", breakdownKey: "receivables" },
  { group: "liability", breakdownKey: "liabilities" },
] as const;

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
  subsectionTitle: { fontSize: 10, fontWeight: 700, marginTop: 10, marginBottom: 4 },
  row: { flexDirection: "row", justifyContent: "space-between", paddingVertical: 2 },
  rowLabel: { color: "#334155" },
  rowValue: { fontWeight: 700 },
  muted: { color: "#64748B" },
  legendRow: { flexDirection: "row", alignItems: "center", marginBottom: 2 },
  legendSwatch: { width: 7, height: 7, marginRight: 4 },
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

        {data.trend.length > 0 && (
          <View wrap={false}>
            <Text style={styles.sectionTitle}>{t("trend.title")}</Text>
            <TrendChart trend={data.trend} />
            <View style={{ marginTop: 4 }}>
              <View style={styles.legendRow}>
                <View
                  style={[styles.legendSwatch, { backgroundColor: TREND_LIVING_EXPENSES_COLOR }]}
                />
                <Text style={styles.muted}>{t("trend.livingExpensesLegend")}</Text>
              </View>
              <View style={styles.legendRow}>
                <View
                  style={[styles.legendSwatch, { backgroundColor: TREND_INVESTMENT_RETURN_COLOR }]}
                />
                <Text style={styles.muted}>{t("trend.investmentReturnLegend")}</Text>
              </View>
            </View>
          </View>
        )}

        <CompositionSection
          title={t("composition.title", { group: t("breakdown.assets") })}
          data={data.composition.assets}
          t={t}
        />
        <CompositionSection
          title={t("composition.title", { group: t("breakdown.investments") })}
          data={data.composition.investments}
          t={t}
        />
        <CompositionSection
          title={t("composition.title", { group: t("breakdown.liabilities") })}
          data={data.composition.liabilities}
          t={t}
        />
        <CompositionSection
          title={t("composition.incomeTitle")}
          data={data.composition.earnedIncome}
          t={t}
          labelKeyPrefix="incomeCategory"
        />
        <CompositionSection
          title={t("composition.returnTitle")}
          data={data.composition.investmentReturn}
          t={t}
        />

        <Text style={styles.sectionTitle}>{t("itemized.title")}</Text>
        {GROUP_ORDER.every(({ group }) => data.itemizedPositions[group].length === 0) ? (
          <Text style={styles.muted}>{t("itemized.empty")}</Text>
        ) : (
          GROUP_ORDER.map(({ group, breakdownKey }) => (
            <ItemizedGroupSection
              key={group}
              title={t(`breakdown.${breakdownKey}`)}
              positions={data.itemizedPositions[group]}
              currency={data.currency}
              t={t}
            />
          ))
        )}
      </Page>
    </Document>
  );
}

// CompositionSection renders one composition donut + legend, or nothing when
// there's no data to show (baseline month for income/return, a group with no
// positions). labelKeyPrefix defaults to "subtype" — the dictionary shared by
// asset/investment/liability subtypes and investment-return instruments; only
// earned-income composition needs its own category dictionary.
function CompositionSection({
  title,
  data,
  t,
  labelKeyPrefix = "subtype",
}: {
  title: string;
  data: CompositionSlice[];
  t: TFunction;
  labelKeyPrefix?: "subtype" | "incomeCategory";
}) {
  if (data.length === 0) return null;
  return (
    <View wrap={false}>
      <Text style={styles.subsectionTitle}>{title}</Text>
      <View style={{ flexDirection: "row", alignItems: "center" }}>
        <PieChart data={data} />
        <View style={{ marginLeft: 12 }}>
          <PieChartLegend
            data={data}
            labelFor={(key, percent) =>
              t("composition.legendLine", { label: t(`${labelKeyPrefix}.${key}`), percent })
            }
          />
        </View>
      </View>
    </View>
  );
}

// ItemizedGroupSection lists every position in one group, largest first
// (reportPdfData.groupItemized's sort). Skips groups with nothing to show —
// the overall "no positions" fallback in ReportDocument handles the all-empty
// case, so each group here can just render nothing rather than an empty
// heading.
function ItemizedGroupSection({
  title,
  positions,
  currency,
  t,
}: {
  title: string;
  positions: ItemizedPosition[];
  currency: string;
  t: TFunction;
}) {
  if (positions.length === 0) return null;
  return (
    <View wrap={false}>
      <Text style={styles.subsectionTitle}>{title}</Text>
      {positions.map((p) => (
        <View key={p.id} style={styles.row}>
          <Text style={styles.rowLabel}>
            {p.name}
            {p.stale ? ` ${t("itemized.carriedForward")}` : ""}
          </Text>
          <Text>{formatCurrency(String(p.amount), currency)}</Text>
        </View>
      ))}
    </View>
  );
}
