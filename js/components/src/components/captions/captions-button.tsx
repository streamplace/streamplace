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
  useCaptionTrackLabel,
  useSetCaptionTrack,
  useToggleCaptions,
} from "./use-captions";

const OFF = "off";

/**
 * The player's CC toggle and track menu, plus caption style settings.
 *
 * `onOpenSettings` replaces the built-in settings dialog, e.g. to
 * navigate to the app's Captions settings page.
 */
export function CaptionsButton({
  dropdownPortalContainer,
  onOpenSettings,
  size,
}: {
  dropdownPortalContainer?: string;
  onOpenSettings?: () => void;
  size?: number;
}) {
  const { t } = useTranslation();
  const { theme } = useTheme();
  const { tracks, track, enabled } = useCaptionSelection();
  const setTrack = useSetCaptionTrack();
  const trackLabel = useCaptionTrackLabel();
  const [settingsOpen, setSettingsOpen] = useState(false);
  const toggle = useToggleCaptions();

  const on = enabled;
  const Icon = on ? Captions : CaptionsOff;
  const openSettings = () => {
    if (onOpenSettings) onOpenSettings();
    else setSettingsOpen(true);
  };

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
                  {tracks.map((tr) => (
                    <DropdownMenuRadioItem
                      key={tr.id}
                      value={tr.id}
                      testID={`player-cc-track-${tr.id}`}
                      closeOnPress
                    >
                      <Text>{trackLabel(tr)}</Text>
                    </DropdownMenuRadioItem>
                  ))}
                </DropdownMenuRadioGroup>
              </DropdownMenuGroup>
              <DropdownMenuGroup>
                <DropdownMenuItem
                  closeOnPress={true}
                  onPress={openSettings}
                  testID="player-cc-settings"
                >
                  <Text>{t("player-captions-style")}</Text>
                </DropdownMenuItem>
              </DropdownMenuGroup>
            </View>
          </ResponsiveDropdownMenuContent>
        </DropdownMenu>
      </View>
      {!onOpenSettings && (
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
      )}
    </>
  );
}
