// Thin wrapper around sonner's `toast()` that mirrors the shape of
// the app's `useToast` from @streamplace/components. Just enough to
// power useBlueskyNotifications on web; a fuller toast system (queue,
// theming, variants) can be added in Phase 7 polish.
//
// The shadcn-style <Toaster /> lives at
// js/web/src/components/ui/sonner.tsx; it must be mounted once at
// the root of the app for any toast() call to render.
import { createElement } from "react";
import { toast as sonnerToast } from "sonner";

interface ToastOptions {
  variant?: "error" | "success" | "info";
  duration?: number;
  actionLabel?: string;
  onAction?: () => void;
  iconLeft?: React.ComponentType<{ className?: string }>;
  // sonner accepts additional options (description, etc.) that
  // callers can pass through. Keep the surface loose here.
  [key: string]: any;
}

export function useToast() {
  return {
    show: (message: string, description?: string, options?: ToastOptions) => {
      const {
        actionLabel,
        onAction,
        iconLeft: IconLeft,
        variant,
        ...rest
      } = options ?? {};
      const toastFn =
        variant === "error"
          ? sonnerToast.error
          : variant === "success"
            ? sonnerToast.success
            : sonnerToast;
      toastFn(message, {
        ...(description !== undefined ? { description } : {}),
        ...(IconLeft
          ? { icon: createElement(IconLeft, { className: "size-4" }) }
          : {}),
        ...(actionLabel && onAction
          ? { action: { label: actionLabel, onClick: onAction } }
          : {}),
        ...rest,
      });
    },
  };
}
