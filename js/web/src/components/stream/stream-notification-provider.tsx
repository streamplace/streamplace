import { createContext, useContext, useEffect, useState } from "react";
import {
  type StreamNotification,
  type StreamNotificationManager,
} from "../../lib/stream-notification";

export const StreamNotificationContext =
  createContext<StreamNotificationManager | null>(null);

type Position = "top" | "bottom" | "top-right";

export function StreamNotificationProvider({
  position = "top",
  importantOnly = false,
}: {
  position?: Position;
  importantOnly?: boolean;
}) {
  const manager = useContext(StreamNotificationContext);
  if (!manager) throw new Error("Stream notifications require a manager");
  const [notifications, setNotifications] = useState(manager.getAll());

  useEffect(() => manager.subscribe(setNotifications), [manager]);

  return (
    <div
      className={`pointer-events-none absolute inset-x-3 z-40 flex flex-col gap-2 ${position === "top-right" ? "top-3 ml-auto max-w-sm" : `mx-auto max-w-xl ${position === "top" ? "top-3" : "bottom-3"}`}`}
    >
      {notifications.map((notification) =>
        importantOnly && !notification.important ? null : (
          <NotificationItem
            key={notification.id}
            notification={notification}
            position={position}
          />
        ),
      )}
    </div>
  );
}

function NotificationItem({
  notification,
  position,
}: {
  notification: StreamNotification;
  position: Position;
}) {
  return (
    <div
      onClick={(event) => event.stopPropagation()}
      className={`animate-in fade-in pointer-events-auto rounded-lg border border-(--color-border) shadow-lg duration-200 motion-reduce:animate-none ${position === "bottom" ? "slide-in-from-bottom-2" : "slide-in-from-top-2"} ${notification.render ? "" : "bg-(--color-bg-elevated) text-(--color-fg)"}`}
    >
      {notification.render ? (
        notification.render()
      ) : (
        <p className="px-3 py-2 text-sm">{notification.message}</p>
      )}
    </div>
  );
}
