import { Card, CardRow } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "../../lib/store";

// Notifications settings: the opt-in surface for web push.
//
// The toggle calls enableWebNotifications (which requests permission,
// subscribes the browser's PushManager, and registers the subscription with
// the backend) or disableWebNotifications (unsubscribes + prunes the server
// row). Ported from js/app/components/settings/notifications-category-settings.tsx.
export const Route = createFileRoute("/settings/notifications")({
  component: NotificationsSettings,
});

function NotificationsSettings() {
  const { t } = useTranslation("settings");
  const enableWebNotifications = useStore(
    (state) => state.enableWebNotifications,
  );
  const disableWebNotifications = useStore(
    (state) => state.disableWebNotifications,
  );
  const webNotificationPermission = useStore(
    (state) => state.webNotificationPermission,
  );
  const notificationToken = useStore((state) => state.notificationToken);

  const [busy, setBusy] = useState(false);

  // The "enabled" state: permission === granted AND we have a registered
  // subscription token.
  const permission = webNotificationPermission();
  const enabled = permission === "granted" && !!notificationToken;

  // Re-check permission when the browser's permission state changes (e.g. the
  // user toggled it in browser settings while the app is open).
  const [, forceRender] = useState(0);
  useEffect(() => {
    if (typeof navigator.permissions === "undefined") return;
    let status: PermissionStatus | undefined;
    navigator.permissions.query({ name: "notifications" }).then((s) => {
      status = s;
      s.onchange = () => forceRender((k) => k + 1);
    });
    return () => {
      if (status) status.onchange = null;
    };
  }, []);

  const handleToggle = async (value: boolean) => {
    if (busy) return;
    setBusy(true);
    try {
      if (value) {
        await enableWebNotifications();
      } else {
        await disableWebNotifications();
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-display text-xl font-semibold">
          {t("notifications")}
        </h1>
        <p className="mt-1 text-sm text-(--color-fg-muted)">
          {t("notifications-web-description")}
        </p>
      </div>

      <Card>
        <CardRow>
          <div className="flex items-center justify-between">
            <div className="pr-4">
              <div className="text-sm font-medium">
                {t("notifications-title")}
              </div>
              <div className="mt-0.5 text-xs text-(--color-fg-muted)">
                {permission === "denied"
                  ? t("notifications-blocked-description")
                  : t("notifications-web-description")}
              </div>
            </div>
            <Switch
              checked={enabled}
              onCheckedChange={handleToggle}
              disabled={busy || permission === "denied"}
            />
          </div>
        </CardRow>
      </Card>

      {permission === "denied" && (
        <p className="text-sm text-(--color-fg-muted)">
          {t("notifications-blocked-help")}
        </p>
      )}
    </div>
  );
}
