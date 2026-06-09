import { cn } from "@/lib/utils";

interface SwitchProps {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  className?: string;
  size?: "sm" | "default";
}

function Switch({
  checked,
  onCheckedChange,
  disabled,
  className,
  size = "default",
}: SwitchProps) {
  const isSmall = size === "sm";
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        "relative inline-flex shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors disabled:opacity-50 disabled:cursor-not-allowed",
        checked ? "bg-info" : "bg-danger",
        isSmall ? "h-4 w-7" : "h-5 w-9",
        className,
      )}
    >
      <span
        className={cn(
          "pointer-events-none inline-block rounded-full bg-white shadow-sm transition-transform",
          isSmall ? "size-3" : "size-4",
          checked
            ? isSmall
              ? "translate-x-3"
              : "translate-x-4"
            : "translate-x-0",
        )}
      />
    </button>
  );
}

export { Switch };
