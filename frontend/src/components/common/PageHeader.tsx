import type { ReactNode } from "react";
import { Link } from "react-router";
import { Button } from "@/components/ui/button";

// The one page header (ADR-0050 amendment, #837). Three variants share one
// layout — back link above (sub-page only), title + subtitle left, actions right,
// top-aligned — and differ in what belongs in `actions`:
//   hub       period control (if any), then at most one primary
//   list      secondary actions, then one primary
//   sub-page  actions after a "← Back" link
// At most one gold (`default` variant) button per header; header buttons take
// the `default` size, so section toolbars (`sm`) never mix with them.
type Back = { label: string; testId?: string } & (
  { to: string; onClick?: never } | { onClick: () => void; to?: never }
);

type Props = {
  title: ReactNode;
  subtitle?: ReactNode;
  actions?: ReactNode;
  titleTestId?: string;
} & ({ variant: "hub" | "list"; back?: never } | { variant: "sub-page"; back: Back });

export function PageHeader({ variant, title, subtitle, actions, titleTestId, back }: Props) {
  return (
    <div data-slot="page-header" data-variant={variant} className="space-y-1">
      {back &&
        (back.to !== undefined ? (
          <Button asChild variant="ghost" className="-ml-2.5" data-testid={back.testId}>
            <Link to={back.to}>{back.label}</Link>
          </Button>
        ) : (
          <Button
            variant="ghost"
            onClick={back.onClick}
            className="-ml-2.5"
            data-testid={back.testId}
          >
            {back.label}
          </Button>
        ))}
      <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between md:gap-4">
        <div className="min-w-0">
          <h1 data-testid={titleTestId} className="text-2xl font-semibold tracking-tight">
            {title}
          </h1>
          {subtitle && <p className="text-sm text-muted-foreground">{subtitle}</p>}
        </div>
        {actions && (
          <div className="flex flex-wrap items-center gap-2 max-md:w-full max-md:[&>*]:flex-1 md:shrink-0 md:justify-end">
            {actions}
          </div>
        )}
      </div>
    </div>
  );
}
