import { Suspense } from "react";
import { lazyWithReload } from "@/lib/lazyWithReload";
import type { FxRate, HouseholdMember, MonthlyReport } from "@/api/types";
import type { Me } from "@/hooks/useSession";

type Props = {
  reports: MonthlyReport[];
  selected: MonthlyReport;
  currency: string;
  secondaryCurrency: string;
  rates: FxRate[];
  members: HouseholdMember[] | undefined;
  me: Me | null | undefined;
};

// Lazy boundary so @react-pdf/renderer lands in a separate chunk, fetched on
// mount rather than bundled into the main dashboard load (ADR-0044).
const ReportPdfButtonImpl = lazyWithReload(() => import("./ReportPdfButtonImpl"));

export function ReportPdfButton(props: Props) {
  return (
    <Suspense fallback={null}>
      <ReportPdfButtonImpl {...props} />
    </Suspense>
  );
}
