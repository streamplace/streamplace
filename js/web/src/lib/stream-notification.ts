import type { ReactNode } from "react";

export type StreamNotificationConfig = {
  id: string;
  message?: string;
  render?: () => ReactNode;
  duration?: number;
};

export type StreamNotification = StreamNotificationConfig & {
  startTime: number;
};

class StreamNotificationManager {
  private notifications: StreamNotification[] = [];
  private listeners = new Set<(notifications: StreamNotification[]) => void>();
  private dismissTimers = new Map<string, ReturnType<typeof setTimeout>>();

  show(config: StreamNotificationConfig) {
    const timer = this.dismissTimers.get(config.id);
    if (timer) {
      clearTimeout(timer);
      this.dismissTimers.delete(config.id);
    }

    this.notifications = this.notifications.filter(
      (notification) => notification.id !== config.id,
    );
    const notification = {
      ...config,
      duration: config.duration ?? 5,
      startTime: Date.now(),
    };
    this.notifications = [...this.notifications, notification];
    this.notifyListeners();

    if (notification.duration > 0) {
      this.dismissTimers.set(
        config.id,
        setTimeout(() => this.hide(config.id), notification.duration * 1000),
      );
    }
  }

  hide(id: string) {
    const timer = this.dismissTimers.get(id);
    if (timer) {
      clearTimeout(timer);
      this.dismissTimers.delete(id);
    }

    const remaining = this.notifications.filter(
      (notification) => notification.id !== id,
    );
    if (remaining.length === this.notifications.length) return;
    this.notifications = remaining;
    this.notifyListeners();
  }

  getAll() {
    return this.notifications;
  }

  subscribe(listener: (notifications: StreamNotification[]) => void) {
    this.listeners.add(listener);
    listener(this.notifications);
    return () => {
      this.listeners.delete(listener);
    };
  }

  private notifyListeners() {
    this.listeners.forEach((listener) => listener(this.notifications));
  }
}

export const streamNotificationManager = new StreamNotificationManager();

export const streamNotification = {
  show: (config: StreamNotificationConfig) =>
    streamNotificationManager.show(config),
  hide: (id: string) => streamNotificationManager.hide(id),
};
