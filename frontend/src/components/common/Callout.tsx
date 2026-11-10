import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

const VARIANTS = {
  info: "border-border bg-muted text-foreground",
  warning: "border-warning/40 bg-warning-surface text-foreground",
} as const;

export type CalloutVariant = keyof typeof VARIANTS;

export function Callout({
  variant = "info",
  className,
  children,
  ...rest
}: {
  variant?: CalloutVariant;
  className?: string;
  children: ReactNode;
  "data-testid"?: string;
}) {
  return (
    <div
      role={variant === "warning" ? "alert" : "note"}
      data-variant={variant}
      className={cn("rounded-md border px-3 py-2 text-xs", VARIANTS[variant], className)}
      {...rest}
    >
      {children}
    </div>
  );
}
