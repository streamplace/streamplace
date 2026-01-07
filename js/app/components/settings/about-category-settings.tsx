import {
  MenuContainer,
  MenuGroup,
  MenuSeparator,
  Text,
  useDanmuUnlocked,
  useSetDanmuUnlocked,
  useTheme,
  useToast,
  View,
  zero,
} from "@streamplace/components";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ScrollView } from "react-native";
import {
  SettingsExternalItem,
  SettingsRowItem,
} from "./components/settings-navigation-item";
import { StreamplaceUpdatesRow, StreamplaceVersionRow } from "./updates";

let buildInfo: {
  hash: string;
  shortHash: string;
  branch: string;
  tag: string;
  isDirty: boolean;
  buildTime: string;
} | null = null;

try {
  buildInfo = require("../../src/build-info.json");
} catch {
  // build-info.json doesn't exist in dev mode
}

const VERSION_REGEX = /^v?(\d+\.\d+\.\d+)(-.+)?$/;
function cutVersionPrefix(version: string) {
  if (VERSION_REGEX.test(version)) {
    const match = VERSION_REGEX.exec(version);
    if (match && match[2]) {
      return match[2].replace(/^-/, "");
    }
  }
  return version;
}

const UNLOCK_TAP_COUNT = 5;
export function AboutCategorySettings() {
  const { t } = useTranslation("settings");
  const theme = useTheme();
  const toast = useToast();

  const [checked, setChecked] = useState(false);
  const [tapCount, setTapCount] = useState(0);
  const danmuUnlocked = useDanmuUnlocked();
  const setDanmuUnlocked = useSetDanmuUnlocked();

  const handleVersionPress = () => {
    if (danmuUnlocked) {
      toast.show("You are already a developer", undefined, {
        duration: 2,
        variant: "info",
        actionLabel: "Stop being a developer",
        onAction: () => {
          setDanmuUnlocked(false);
          toast.show("You are no longer a developer", undefined, {
            duration: 2,
            variant: "info",
          });
        },
      });
      return;
    }

    const newCount = tapCount + 1;
    setTapCount(newCount);

    if (newCount >= UNLOCK_TAP_COUNT) {
      setDanmuUnlocked(true);
      toast.show("You are now a developer", "have fun! lol", {
        duration: 20,
        variant: "success",
      });
      setTapCount(0);
    }
  };

  const getBuildStatus = () => {
    if (!buildInfo) {
      return "dev";
    }
    return buildInfo.isDirty || process?.env.NODE_ENV === "development"
      ? "dev"
      : "prod";
  };

  const buildLabel = buildInfo ? buildInfo.tag : "development";
  const buildStatus = getBuildStatus();

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <MenuContainer>
            <MenuGroup>
              <StreamplaceVersionRow />
              <MenuSeparator />
              <SettingsRowItem onPress={handleVersionPress}>
                <View style={{ flex: 1 }}>
                  <Text size="lg">Build</Text>
                </View>
                <View style={{ alignItems: "flex-end" }}>
                  <Text size="lg" color="muted">
                    {cutVersionPrefix(buildLabel)} ({buildStatus})
                  </Text>
                </View>
              </SettingsRowItem>
              {buildInfo && buildInfo.branch !== "main" && (
                <>
                  <MenuSeparator />
                  <SettingsRowItem onPress={handleVersionPress}>
                    <View style={{ flex: 1 }}>
                      <Text size="lg">Branch</Text>
                    </View>
                    <View style={{ alignItems: "flex-end" }}>
                      <Text size="lg" color="muted">
                        {buildInfo ? buildInfo.branch : "development"}
                      </Text>
                    </View>
                  </SettingsRowItem>
                </>
              )}
              <StreamplaceUpdatesRow />
            </MenuGroup>

            <MenuGroup>
              <SettingsExternalItem
                title="Terms of Service"
                link="OpenSourceLicenses"
              />
              <MenuSeparator />
              <SettingsExternalItem
                title="Privacy Policy"
                link="OpenSourceLicenses"
              />
            </MenuGroup>
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}
