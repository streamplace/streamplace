import {
  MenuContainer,
  MenuGroup,
  Text,
  View,
  zero,
} from "@streamplace/components";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Platform, ScrollView } from "react-native";
import { useStore } from "store";
import { SettingToggle } from "./components/setting-toggle";

// NotificationsCategorySettings is the opt-in surface for push notifications.
//
// On web, the toggle calls enableWebNotifications (which requests permission,
// subscribes the browser's PushManager, and registers the subscription with the
// backend) or disableWebNotifications (unsubscribes + prunes the server row).
//
// On native, push is driven by initPushNotifications at app start (FCM), so
// the toggle reflects the OS permission state and, when supported, links the
// user to the system settings to change it. The toggle itself can't grant
// permission on native — the OS prompt already ran at startup — but it shows
// the current state honestly.
export function NotificationsCategorySettings() {
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

  const isWeb = Platform.OS === "web";
  const [busy, setBusy] = useState(false);

  // The "enabled" state: on web it's permission === granted AND we have a
  // registered subscription token. On native it's whether the OS granted
  // permission (the FCM token is acquired separately at startup).
  const permission = isWeb ? webNotificationPermission() : "granted";
  const enabled = isWeb
    ? permission === "granted" && !!notificationToken
    : permission === "granted";

  // Re-check permission when the browser's permission state changes (e.g. the
  // user toggled it in browser settings while the app is open).
  const [, forceRender] = useState(0);
  useEffect(() => {
    if (!isWeb || typeof navigator.permissions === "undefined") return;
    let status: PermissionStatus | undefined;
    navigator.permissions.query({ name: "notifications" }).then((s) => {
      status = s;
      s.onchange = () => forceRender((k) => k + 1);
    });
    return () => {
      if (status) status.onchange = null;
    };
  }, [isWeb]);

  const handleToggle = async (value: boolean) => {
    if (busy) return;
    setBusy(true);
    try {
      if (value) {
        if (isWeb) {
          await enableWebNotifications();
        }
        // native: permission was already requested at startup; nothing to do
      } else {
        if (isWeb) {
          await disableWebNotifications();
        }
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <MenuContainer>
            <MenuGroup>
              <SettingToggle
                title={t("notifications-title")}
                description={
                  isWeb
                    ? permission === "denied"
                      ? t("notifications-blocked-description")
                      : t("notifications-web-description")
                    : t("notifications-mobile-description")
                }
                value={enabled}
                onValueChange={handleToggle}
              />
            </MenuGroup>
            {isWeb && permission === "denied" && (
              <View style={[zero.px[3], zero.py[2]]}>
                <Text size="sm" color="muted">
                  {t("notifications-blocked-help")}
                </Text>
              </View>
            )}
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}
