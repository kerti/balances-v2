import { Skeleton } from "@/components/ui/skeleton";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

// Suspense fallback for the lazy chart chunks. Callers pass the same height
// class the loaded chart's container uses, so the card does not shift when the
// chart arrives. No shimmer under prefers-reduced-motion.
export function ChartSkeleton({ className }: { className: string }) {
  const { t } = useTranslation("common");
  return (
    <Skeleton
      role="status"
      aria-busy="true"
      aria-label={t("loading")}
      data-testid="chart-skeleton"
      className={cn("w-full motion-reduce:animate-none", className)}
    />
  );
}
