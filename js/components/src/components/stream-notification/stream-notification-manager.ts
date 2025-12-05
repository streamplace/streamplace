export type NotificationConfig = {
  id?: string;
  message?: string;
  render?: (
    isExiting: boolean,
    onDismiss: () => void,
    startTime?: number,
  ) => React.ReactNode;
  duration?: number; // seconds, 0 = manual dismiss only
  actionLabel?: string;
  onAction?: () => void;
  onDismiss?: () => void;
  onUserDismiss?: () => void;
  onAutoDismiss?: () => void;
  variant?: "default" | "info" | "warning";
};

export type StreamNotification = NotificationConfig & {
  id: string;
  visible: boolean;
  shouldDismiss?: boolean;
  dismissReason?: "user" | "auto";
  startTime?: number;
};

type Listener = (notifications: StreamNotification[]) => void;

class StreamNotificationManager {
  private notifications: StreamNotification[] = [];
  private listeners: Set<Listener> = new Set();
  private dismissTimers: Map<string, NodeJS.Timeout> = new Map();

  show(config: NotificationConfig) {
    const notification: StreamNotification = {
      id: config.id || `notification-${Date.now()}`,
      message: config.message,
      render: config.render,
      duration: config.duration ?? 5,
      actionLabel: config.actionLabel,
      onAction: config.onAction,
      onDismiss: config.onDismiss,
      onUserDismiss: config.onUserDismiss,
      onAutoDismiss: config.onAutoDismiss,
      variant: config.variant ?? "default",
      visible: true,
      startTime: Date.now(),
    };

    // if notification with same ID exists, dismiss it first
    const existingIndex = this.notifications.findIndex(
      (n) => n.id === notification.id,
    );
    if (existingIndex !== -1) {
      const existingTimer = this.dismissTimers.get(notification.id);
      if (existingTimer) {
        clearTimeout(existingTimer);
        this.dismissTimers.delete(notification.id);
      }
      this.notifications = this.notifications.filter(
        (n) => n.id !== notification.id,
      );
    }

    this.notifications = [...this.notifications, notification];
    this.notifyListeners();

    // auto-dismiss if duration > 0
    if (notification.duration && notification.duration > 0) {
      const timer = setTimeout(() => {
        this.requestDismiss(notification.id, "auto");
      }, notification.duration * 1000);
      this.dismissTimers.set(notification.id, timer);
    }
  }

  requestDismiss(id: string, reason: "user" | "auto" = "user") {
    const notification = this.notifications.find((n) => n.id === id);
    if (!notification) {
      console.log("Notification not found!");
      return;
    }

    // mark the notification for dismissal
    notification.shouldDismiss = true;
    notification.dismissReason = reason;
    this.notifyListeners();
  }

  hide(id: string, reason: "user" | "auto" = "user") {
    console.log("Hide called with id:", id, "reason:", reason);
    console.log(
      "Current notifications:",
      this.notifications.map((n) => n.id),
    );
    const notification = this.notifications.find((n) => n.id === id);
    if (!notification) {
      console.log("Notification not found!");
      return;
    }

    const timer = this.dismissTimers.get(id);
    if (timer) {
      clearTimeout(timer);
      this.dismissTimers.delete(id);
    }

    this.notifications = this.notifications.filter((n) => n.id !== id);
    console.log(
      "Remaining notifications:",
      this.notifications.map((n) => n.id),
    );
    this.notifyListeners();

    notification.onDismiss?.();
    if (reason === "user") {
      notification.onUserDismiss?.();
    } else {
      notification.onAutoDismiss?.();
    }
  }

  getAll(): StreamNotification[] {
    return this.notifications;
  }

  subscribe(listener: Listener) {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  }

  private notifyListeners() {
    this.listeners.forEach((listener) => {
      listener(this.notifications);
    });
  }
}

export const streamNotificationManager = new StreamNotificationManager();

export const streamNotification = {
  show: (config: NotificationConfig) => streamNotificationManager.show(config),
  hide: (id: string) => streamNotificationManager.hide(id),
};
