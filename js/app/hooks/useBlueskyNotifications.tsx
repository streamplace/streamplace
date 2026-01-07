import { useToast } from "@streamplace/components";
import { CircleX } from "lucide-react-native";
import { useEffect } from "react";
import { useStore } from "../store";

function titleCase(str: string) {
  return str
    .split(" ")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

/**
 * hook to listen for bluesky notifications and display them as toasts
 * call this hook once at the app level
 */
export function useBlueskyNotifications() {
  let toast = useToast();
  const notification = useStore((state) => state.notification);
  const clearNotification = useStore((state) => state.clearNotification);

  useEffect(() => {
    if (notification) {
      // if it's a missing params notification, we'll want to parse the message properly
      // e.g. Missing params, got: https://iori.kiryu.cloud/login?error=authorize_failed&error_description=code%3D400%2C+message%3Dfailed+to+resolve+handle+%27https%3A%2F%2Fselfhosted.social%27%3A+handle+syntax+didn%27t+validate+via+regex%3A+https%3A%2F%2Fselfhosted.social
      if (notification.message.startsWith("Missing params, got")) {
        const urlPart = notification.message.replace(
          "Missing params, got: ",
          "",
        );
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
              duration: 100,
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
        } catch (e) {
          // fallback if URL parsing fails
          toast.show(
            notification.type === "success"
              ? "Congrats!"
              : "An issue occured when logging in",
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
            : "An issue occured when logging in",
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
      // clears the notification in the store after showing
      clearNotification();
    }
  }, [notification, clearNotification]);
}
