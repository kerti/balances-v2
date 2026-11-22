import * as React from "react";
import { Label as LabelPrimitive } from "radix-ui";

import { cn } from "@/lib/utils";

type LabelProps = React.ComponentProps<typeof LabelPrimitive.Root> & {
  /**
   * Marks the field as required with a muted trailing "*" (#953 S12; red is for
   * debt and negatives only, ADR-0054). Drawn by CSS
   * so the label's text (and accessible name) is unchanged; the control itself
   * carries the native `required` that assistive tech reads.
   */
  required?: boolean;
};

function Label({ className, required, ...props }: LabelProps) {
  return (
    <LabelPrimitive.Root
      data-slot="label"
      data-required={required ? "true" : undefined}
      className={cn(
        "flex items-center gap-2 text-sm leading-none font-medium select-none group-data-[disabled=true]:pointer-events-none group-data-[disabled=true]:opacity-50 peer-disabled:cursor-not-allowed peer-disabled:opacity-50",
        required && "after:-ml-1.5 after:text-muted-foreground after:content-['*'_/_'']",
        className,
      )}
      {...props}
    />
  );
}

export { Label };
