import * as React from "react";
import { cn } from "../../lib/utils";

function HoverCard({
  children,
  openDelay = 500,
  closeDelay = 200,
  trigger = "hover",
}: {
  children: React.ReactNode;
  openDelay?: number;
  closeDelay?: number;
  trigger?: "hover" | "click";
}) {
  const [open, setOpen] = React.useState(false);
  const openTimeout = React.useRef<ReturnType<typeof setTimeout>>(undefined);
  const closeTimeout = React.useRef<ReturnType<typeof setTimeout>>(undefined);
  const containerRef = React.useRef<HTMLDivElement>(null);

  const show = React.useCallback(() => {
    clearTimeout(closeTimeout.current);
    openTimeout.current = setTimeout(() => setOpen(true), openDelay);
  }, [openDelay]);

  const hide = React.useCallback(() => {
    clearTimeout(openTimeout.current);
    closeTimeout.current = setTimeout(() => setOpen(false), closeDelay);
  }, [closeDelay]);

  const toggle = React.useCallback(() => setOpen((o) => !o), []);
  const close = React.useCallback(() => setOpen(false), []);

  React.useEffect(() => {
    return () => {
      clearTimeout(openTimeout.current);
      clearTimeout(closeTimeout.current);
    };
  }, []);

  // In click mode, close on outside pointerdown or Escape.
  React.useEffect(() => {
    if (trigger !== "click" || !open) return;
    const onPointerDown = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target as Node)
      ) {
        close();
      }
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") close();
    };
    document.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [trigger, open, close]);

  return (
    <div
      ref={containerRef}
      className="relative inline-flex"
      {...(trigger === "hover"
        ? { onMouseEnter: show, onMouseLeave: hide }
        : {})}
    >
      {React.Children.map(children, (child) => {
        if (!React.isValidElement(child)) return child;
        if (child.type === HoverCardTrigger) {
          if (trigger === "click") {
            return React.cloneElement(child as React.ReactElement<any>, {
              onClick: toggle,
              role: "button",
              tabIndex: 0,
              onKeyDown: (e: React.KeyboardEvent) => {
                if (e.key === "Enter" || e.key === " ") {
                  e.preventDefault();
                  toggle();
                } else if (e.key === "Escape") {
                  close();
                }
              },
            });
          }
          return React.cloneElement(child as React.ReactElement<any>, {});
        }
        if (child.type === HoverCardContent) {
          if (!open) return null;
          return React.cloneElement(child as React.ReactElement<any>, {});
        }
        return child;
      })}
    </div>
  );
}

function HoverCardTrigger({
  children,
  className,
  ...props
}: React.ComponentProps<"span">) {
  return (
    <span className={cn("cursor-default", className)} {...props}>
      {children}
    </span>
  );
}

function HoverCardContent({
  children,
  className,
  side = "top",
  align = "center",
  ...props
}: React.ComponentProps<"div"> & {
  side?: "top" | "bottom" | "left" | "right";
  align?: "start" | "center" | "end";
}) {
  return (
    <div
      className={cn(
        "absolute z-50 w-64 rounded-lg border border-(--color-border) bg-(--color-bg-elevated) shadow-lg",
        side === "top" && "bottom-full mb-2",
        side === "bottom" && "top-full mt-2",
        align === "center" && "left-1/2 -translate-x-1/2",
        align === "start" && "left-0",
        align === "end" && "right-0",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export { HoverCard, HoverCardContent, HoverCardTrigger };
