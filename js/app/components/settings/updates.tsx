import { Text } from "@streamplace/components";
import { useTranslation } from "react-i18next";
import pkg from "../../package.json";
import { SettingsRowItem } from "./components/settings-navigation-item";

export function StreamplaceVersionRow() {
  const { t } = useTranslation("settings");

  return (
    <SettingsRowItem>
      <Text size="lg">{t("app-version", { version: pkg.version })}</Text>
    </SettingsRowItem>
  );
}

export function StreamplaceUpdatesRow() {
  return null;
}
