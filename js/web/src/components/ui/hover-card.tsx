"use client";

import { Popover as PopoverPrimitive } from "@base-ui/react/popover";
import * as React from "react";
import { cn } from "../../lib/utils";

const HoverCardContext = React.createContext({
  openOnHover: true,
  openDelay: 500,
  closeDelay: 200,
});

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
  return (
    <HoverCardContext.Provider
      value={{ openOnHover: trigger === "hover", openDelay, closeDelay }}
    >
      <PopoverPrimitive.Root>{children}</PopoverPrimitive.Root>
    </HoverCardContext.Provider>
  );
}

function HoverCardTrigger({
  children,
  className,
  ...props
}: React.ComponentProps<"span">) {
  const { openOnHover, openDelay, closeDelay } =
    React.useContext(HoverCardContext);

  return (
    <PopoverPrimitive.Trigger
      nativeButton={false}
      openOnHover={openOnHover}
      delay={openDelay}
      closeDelay={closeDelay}
      render={
        <span className={cn("cursor-default", className)} {...props}>
          {children}
        </span>
      }
    />
  );
}

function HoverCardContent({
  children,
  className,
  side = "top",
  align = "center",
  ...props
}: PopoverPrimitive.Popup.Props &
  Pick<
    PopoverPrimitive.Positioner.Props,
    "align" | "alignOffset" | "side" | "sideOffset"
  >) {
  const { alignOffset = 0, sideOffset = 8, ...popupProps } = props;

  return (
    <PopoverPrimitive.Portal>
      <PopoverPrimitive.Positioner
        side={side}
        sideOffset={sideOffset}
        align={align}
        alignOffset={alignOffset}
        collisionPadding={8}
        collisionAvoidance={{
          side: "flip",
          align: "shift",
          fallbackAxisSide: "none",
        }}
        className="isolate z-50"
      >
        <PopoverPrimitive.Popup
          className={cn(
            "z-50 w-64 max-w-[calc(100vw-1rem)] rounded-lg border border-(--color-border) bg-(--color-bg-elevated) shadow-lg outline-none",
            className,
          )}
          {...popupProps}
        >
          {children}
        </PopoverPrimitive.Popup>
      </PopoverPrimitive.Positioner>
    </PopoverPrimitive.Portal>
  );
}

export { HoverCard, HoverCardContent, HoverCardTrigger };
