// Listen for bluesky-side errors (OAuth failures, login issues) and
// surface them as toasts. Call this once at the app level. Port of
// js/app/hooks/useBlueskyNotifications.tsx. Uses the web's sonner
// toast wrapper.
import { CircleX } from "lucide-react";
import { useEffect } from "react";
import { useStore } from "../lib/store";
import { useToast } from "./use-toast";

function titleCase(str: string) {
  return str
    .split(" ")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

/**
 * Call once near the root of the app. The slice accumulates errors in
 * `state.notification`; this hook renders the toast and clears the
 * notification.
 */
export function useBlueskyNotifications() {
  const toast = useToast();
  const notification = useStore((state) => state.notification);
  const clearNotification = useStore((state) => state.clearNotification);

  useEffect(() => {
    if (!notification) return;

    // Missing-params notifications pack the failing URL into the message.
    if (notification.message.startsWith("Missing params, got")) {
      const urlPart = notification.message.replace("Missing params, got: ", "");
      try {
        const url = new URL(urlPart);
        const error = url.searchParams.get("error") || "Unknown error";
        const errorDescription =
          url.searchParams.get("error_description") || "No description";
        toast.show(
          notification.type === "success"
            ? "Congrats!"
            : "Login issue: " + titleCase(error.replace("_", " ")),
          `${decodeURIComponent(errorDescription)}`,
          {
            duration: 8000,
            variant: notification.type,
            actionLabel: "Copy message",
            iconLeft: CircleX,
            onAction: () => {
              navigator.clipboard.writeText(
                `${error}: ${decodeURIComponent(errorDescription)}`,
              );
            },
          },
        );
      } catch {
        toast.show(
          notification.type === "success"
            ? "Congrats!"
            : "An issue occurred when logging in",
          notification.message,
          {
            variant: notification.type,
            actionLabel: "Copy message",
            onAction: () => {
              navigator.clipboard.writeText(notification.message);
            },
          },
        );
      }
    } else {
      toast.show(
        notification.type === "success"
          ? "Congrats!"
          : "An issue occurred when logging in",
        notification.message,
        {
          variant: notification.type,
          actionLabel: "Copy message",
          onAction: () => {
            navigator.clipboard.writeText(notification.message);
          },
        },
      );
    }
    clearNotification();
  }, [notification, clearNotification, toast]);
}
