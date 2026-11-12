import { InfoHint } from "@/components/common/InfoHint";
import type { InfoField } from "@/components/detail/types";

// The shared info-card body (ADR-0051). Lays out label/value pairs and owns the
// responsive reflow. It **never inspects field identity or domain meaning**:
// alignment/formatting ride on the value node itself, so the grid never learns
// what "numeric" means. Renders nothing for an empty field list.
//
// Columns follow the *card's* width, not the viewport's (#839): the grid is its
// own CSS container, so a card squeezed beside a sidebar at 768px or a dialog
// reflows the same way a phone does. 1 column under 28rem (a label/value row:
// label left, value hard right), 2 from 28rem, 3 from 42rem (each cell: label
// above its value) — a long value wraps inside its cell and can never run into a
// neighbour.
export function InfoGrid({ fields }: { fields: InfoField[] }) {
  if (fields.length === 0) return null;
  return (
    <div className="@container" data-testid="info-grid">
      <dl className="grid grid-cols-1 gap-x-8 gap-y-2 text-sm @md:grid-cols-2 @2xl:grid-cols-3 @md:gap-y-3">
        {fields.map((field, i) => (
          <div
            key={i}
            className="flex min-w-0 items-baseline justify-between gap-3 @md:flex-col @md:items-start @md:justify-start @md:gap-0.5"
          >
            {/* Labels render without a trailing colon — some i18n values still
                carry one from when they doubled as inline prose, but the label/value
                grid reads cleaner without it. */}
            <dt className="text-muted-foreground">
              {field.label.replace(/:\s*$/, "")}
              {field.hint && (
                <InfoHint label={field.label} testId={`field-hint-${i}`} className="-my-3 ml-1">
                  {field.hint}
                </InfoHint>
              )}
            </dt>
            <dd className="flex min-w-0 justify-end text-right break-words @md:justify-start @md:text-left">
              {field.value}
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
