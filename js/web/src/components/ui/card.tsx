import * as React from "react";

import { cn } from "@/lib/utils";

function Card({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      className={cn(
        "rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)]",
        className,
      )}
      {...props}
    />
  );
}

function CardMenuSection({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      className={cn(
        "rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] divide-y divide-[var(--color-border)]",
        className,
      )}
      {...props}
    />
  );
}

function CardRow({ className, ...props }: React.ComponentProps<"div">) {
  return <div className={cn("px-3 py-2.5", className)} {...props} />;
}

export { Card, CardMenuSection, CardRow };
