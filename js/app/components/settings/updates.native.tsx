import {
  Loader,
  MenuSeparator,
  Text,
  useTheme,
  useToast,
  View,
  zero,
} from "@streamplace/components";
import * as ExpoUpdates from "expo-updates";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Platform, TouchableOpacity } from "react-native";
import pkg from "../../package.json";
import { SettingsRowItem } from "./components/settings-navigation-item";

export function StreamplaceVersionRow() {
  const version = pkg.version;
  const updateInfo = ExpoUpdates.useUpdates();
  const { currentlyRunning } = updateInfo;
  const { t } = useTranslation("settings");

  let runTypeMessage = currentlyRunning.isEmbeddedLaunch
    ? t("bundled-runtype")
    : t("ota-runtype");
  if (currentlyRunning.isEmergencyLaunch) {
    runTypeMessage = t("recovery-runtype");
  }

  return (
    <SettingsRowItem>
      <View
        style={[
          zero.layout.flex.row,
          zero.layout.flex.alignCenter,
          zero.layout.flex.justify.between,
          { flex: 1 },
        ]}
      >
        <Text size="lg" style={[{ fontWeight: "semibold" }]}>
          Streamplace v{version}
        </Text>
        <Text size="lg" color="muted">
          {runTypeMessage}
        </Text>
      </View>
    </SettingsRowItem>
  );
}

export function StreamplaceUpdatesRow() {
  const theme = useTheme();
  const updateInfo = ExpoUpdates.useUpdates();
  const { isUpdateAvailable, isUpdatePending } = updateInfo;
  const toast = useToast();
  const { t } = useTranslation("settings");

  console.log(`updateInfo: ${JSON.stringify(updateInfo)}`);

  type CheckStatus = "not_checked" | "checking" | "checked";
  const [checkStatus, setCheckStatus] = useState<CheckStatus>("not_checked");

  useEffect(() => {
    if (isUpdateAvailable && checkStatus === "checked") {
      (async () => {
        try {
          await ExpoUpdates.fetchUpdateAsync();
        } catch (e) {
          setCheckStatus("not_checked");
          toast.show(
            t("modal-update-failed-title"),
            t("modal-update-failed-description", {
              store: Platform.OS === "ios" ? "App Store" : "Play Store",
            }),
          );
        }
      })();
    }
  }, [isUpdateAvailable, checkStatus]);

  const buttonText = isUpdateAvailable
    ? t("download-new-update")
    : t("check-for-updates");

  return (
    <>
      {/* menu separator here because we can't check in the above component */}
      <MenuSeparator />
      <SettingsRowItem
        onPress={async () => {
          try {
            setCheckStatus("checking");
            const res = await ExpoUpdates.checkForUpdateAsync();
            if (!res.isAvailable) {
              setCheckStatus("not_checked");
              toast.show(
                t("modal-latest-version"),
                t("modal-no-update-available"),
                { duration: 2000 },
              );
            } else {
              setCheckStatus("checked");
              toast.show(
                t("modal-update-available-title"),
                t("modal-update-available-description"),
                { duration: 2000 },
              );
            }
          } catch (e) {
            setCheckStatus("not_checked");
            toast.show(
              t("modal-update-failed-title"),
              t("modal-update-failed-description", {
                store: Platform.OS === "ios" ? "App Store" : "Play Store",
              }),
            );
          }
        }}
      >
        <Text size="lg" color="primary">
          {buttonText}
        </Text>
        {checkStatus === "checking" && (
          <Loader color={theme.theme.colors.mutedForeground} />
        )}
      </SettingsRowItem>
      {isUpdatePending && (
        <TouchableOpacity
          style={[
            {
              marginTop: 8,
              backgroundColor: "#007AFF",
              borderRadius: 8,
              paddingHorizontal: 16,
              paddingVertical: 12,
              alignItems: "center",
            },
          ]}
          onPress={() => {
            ExpoUpdates.reloadAsync();
          }}
        >
          <Text color="secondary" style={[{ fontWeight: "600" }]}>
            {t("button-reload-app-on-update")}
          </Text>
        </TouchableOpacity>
      )}
    </>
  );
}
