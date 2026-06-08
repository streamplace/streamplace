import * as React from "react";
import { cn } from "../../lib/utils";

type PopoverContextValue = {
  open: boolean;
  setOpen: (open: boolean) => void;
};

const PopoverContext = React.createContext<PopoverContextValue | null>(null);

function usePopover() {
  const ctx = React.useContext(PopoverContext);
  if (!ctx) throw new Error("Popover components must be used within <Popover>");
  return ctx;
}

function Popover({
  children,
  open: openProp,
  onOpenChange,
}: {
  children: React.ReactNode;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}) {
  const [internalOpen, setInternalOpen] = React.useState(false);
  const open = openProp ?? internalOpen;
  const setOpen = onOpenChange ?? setInternalOpen;

  const value = React.useMemo(() => ({ open, setOpen }), [open, setOpen]);

  return (
    <PopoverContext.Provider value={value}>
      <div className="relative inline-flex">{children}</div>
    </PopoverContext.Provider>
  );
}

function PopoverTrigger({
  children,
  asChild,
}: {
  children: React.ReactNode;
  asChild?: boolean;
}) {
  const { setOpen } = usePopover();
  const child = React.Children.only(children) as React.ReactElement;

  return React.cloneElement(child, {
    onFocus: (e: React.FocusEvent) => {
      (child.props as any).onFocus?.(e);
      setOpen(true);
    },
    onBlur: (e: React.FocusEvent) => {
      (child.props as any).onBlur?.(e);
      setTimeout(() => setOpen(false), 150);
    },
  } as any);
}

function PopoverContent({
  children,
  className,
  align = "start",
  side = "bottom",
  ...props
}: React.ComponentProps<"div"> & {
  align?: "start" | "center" | "end";
  side?: "top" | "bottom";
}) {
  const { open } = usePopover();

  if (!open) return null;

  return (
    <div
      className={cn(
        "absolute z-50 min-w-[12rem] overflow-hidden rounded-md border border-[var(--color-border)] bg-[var(--color-bg-elevated)] shadow-md",
        side === "bottom" && "top-full mt-1",
        side === "top" && "bottom-full mb-1",
        align === "start" && "left-0",
        align === "center" && "left-1/2 -translate-x-1/2",
        align === "end" && "right-0",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export { Popover, PopoverContent, PopoverTrigger };
