import * as React from "react";
import { cn } from "../../lib/utils";

function Separator({
  className,
  orientation = "horizontal",
  decorative = true,
  ...props
}: React.ComponentProps<"hr"> & {
  orientation?: "horizontal" | "vertical";
  decorative?: boolean;
}) {
  return (
    <hr
      role={decorative ? "none" : "separator"}
      aria-orientation={decorative ? undefined : orientation}
      data-orientation={orientation}
      className={cn(
        "shrink-0 border-none",
        orientation === "horizontal"
          ? "h-px w-full bg-(--color-border)"
          : "h-full w-px bg-(--color-border)",
        className,
      )}
      {...props}
    />
  );
}

export { Separator };
