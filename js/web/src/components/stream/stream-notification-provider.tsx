import { useEffect, useState } from "react";
import {
  type StreamNotification,
  type StreamNotificationManager,
} from "../../lib/stream-notification";

export function StreamNotificationProvider({
  manager,
  position = "top",
}: {
  manager: StreamNotificationManager;
  position?: "top" | "bottom";
}) {
  const [notifications, setNotifications] = useState(manager.getAll());

  useEffect(() => manager.subscribe(setNotifications), [manager]);

  return (
    <div
      className={`pointer-events-none absolute inset-x-3 z-40 mx-auto flex max-w-xl flex-col gap-2 ${position === "top" ? "top-3" : "bottom-3"}`}
    >
      {notifications.map((notification) => (
        <NotificationItem
          key={notification.id}
          notification={notification}
          position={position}
        />
      ))}
    </div>
  );
}

function NotificationItem({
  notification,
  position,
}: {
  notification: StreamNotification;
  position: "top" | "bottom";
}) {
  return (
    <div
      className={`animate-in fade-in pointer-events-auto rounded-lg border border-(--color-border) shadow-lg duration-200 motion-reduce:animate-none ${position === "top" ? "slide-in-from-top-2" : "slide-in-from-bottom-2"} ${notification.render ? "" : "bg-(--color-bg-elevated) text-(--color-fg)"}`}
    >
      {notification.render ? (
        notification.render()
      ) : (
        <p className="px-3 py-2 text-sm">{notification.message}</p>
      )}
    </div>
  );
}
