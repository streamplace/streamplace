import { captionLanguageName } from "@streamplace/core";
import { Captions, CaptionsOff, ChevronDown } from "lucide-react-native";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Platform, Pressable, ScrollView, View } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import { ResponsiveDialog } from "../ui/dialog";
import {
  DropdownMenu,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
} from "../ui/dropdown";
import { Text } from "../ui/text";
import { CaptionSettings } from "./caption-settings";
import {
  useCaptionSelection,
  useSetCaptionTrack,
  useToggleCaptions,
} from "./use-captions";

const OFF = "off";

/** The player's CC toggle and track menu, plus caption style settings. */
export function CaptionsButton({
  dropdownPortalContainer,
  size,
}: {
  dropdownPortalContainer?: string;
  size?: number;
}) {
  const { t, i18n } = useTranslation();
  const { theme } = useTheme();
  const { tracks, track, enabled } = useCaptionSelection();
  const setTrack = useSetCaptionTrack();
  const [settingsOpen, setSettingsOpen] = useState(false);
  const toggle = useToggleCaptions();

  const on = enabled;
  const Icon = on ? Captions : CaptionsOff;

  return (
    <>
      <View style={{ flexDirection: "row", alignItems: "center" }}>
        <Pressable
          testID="player-cc-button"
          accessibilityRole={Platform.OS === "web" ? "button" : "switch"}
          accessibilityLabel={t("player-captions")}
          aria-pressed={Platform.OS === "web" ? on : undefined}
          accessibilityState={
            Platform.OS === "web" ? undefined : { checked: on }
          }
          onPress={toggle}
          style={{
            minWidth: theme.touchTargets.minimum,
            minHeight: theme.touchTargets.minimum,
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Icon size={size} color={theme.colors.foreground} />
        </Pressable>
        <DropdownMenu key={dropdownPortalContainer}>
          <DropdownMenuTrigger
            testID="player-cc-menu-button"
            accessibilityRole="button"
            accessibilityLabel={t("player-captions-menu")}
            style={{
              minWidth: theme.touchTargets.minimum,
              minHeight: theme.touchTargets.minimum,
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <ChevronDown size={size} color={theme.colors.foreground} />
          </DropdownMenuTrigger>
          <ResponsiveDropdownMenuContent
            side="top"
            align="end"
            portalHost={dropdownPortalContainer}
          >
            <View testID="player-cc-menu">
              <DropdownMenuGroup title={t("player-captions")}>
                <DropdownMenuRadioGroup
                  value={on && track ? track.id : OFF}
                  onValueChange={(id: string) => {
                    setTrack(tracks.find((tr) => tr.id === id) ?? null);
                  }}
                >
                  <DropdownMenuRadioItem
                    value={OFF}
                    testID="player-cc-track-off"
                    closeOnPress
                  >
                    <Text>{t("player-captions-off")}</Text>
                  </DropdownMenuRadioItem>
                  {tracks.map((tr) => {
                    const name = captionLanguageName(
                      tr.language,
                      i18n.language,
                    );
                    const base =
                      name === tr.language && tr.label ? tr.label : name;
                    return (
                      <DropdownMenuRadioItem
                        key={tr.id}
                        value={tr.id}
                        testID={`player-cc-track-${tr.id}`}
                        closeOnPress
                      >
                        <Text>
                          {tr.source === "auto"
                            ? t("player-captions-track-auto", {
                                language: base,
                              })
                            : base}
                        </Text>
                      </DropdownMenuRadioItem>
                    );
                  })}
                </DropdownMenuRadioGroup>
              </DropdownMenuGroup>
              <DropdownMenuGroup>
                <DropdownMenuItem
                  closeOnPress={true}
                  onPress={() => setSettingsOpen(true)}
                  testID="player-cc-settings"
                >
                  <Text>{t("player-captions-style")}</Text>
                </DropdownMenuItem>
              </DropdownMenuGroup>
            </View>
          </ResponsiveDropdownMenuContent>
        </DropdownMenu>
      </View>
      <ResponsiveDialog
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        title={t("player-captions-style")}
        showCloseButton
        variant="default"
        size="md"
      >
        <ScrollView>
          <CaptionSettings />
        </ScrollView>
      </ResponsiveDialog>
    </>
  );
}
