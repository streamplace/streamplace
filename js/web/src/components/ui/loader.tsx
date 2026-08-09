import type { CSSProperties } from "react";
import { cn } from "../../lib/utils";

export function Loader({
  label,
  className,
  length = 12,
}: {
  label: string;
  className?: string;
  length?: number;
}) {
  let items = Array.from({ length }, (_, index) => index + 1);
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={label}
      className={cn("ui-loader", className)}
      style={{ filter: "drop-shadow(30px 10px 4px #fff);" }}
    >
      {items.map((index) => (
        <span
          key={index}
          aria-hidden="true"
          className="ui-loader__item"
          style={
            {
              "--loader-angle": `${index * 30}deg`,
              "--loader-delay": `${(-((index - 1) / 18)).toFixed(10)}s`,
            } as CSSProperties
          }
        />
      ))}
    </div>
  );
}
