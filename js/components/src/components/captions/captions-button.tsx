import { Captions, CaptionsOff } from "lucide-react-native";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { ScrollView, View } from "react-native";
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
} from "./use-captions";

const OFF = "off";

/**
 * The player's CC button: a menu to turn captions off or pick a track,
 * and a way into the caption style settings. Hidden when the stream or
 * video has no caption tracks.
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

  if (tracks.length === 0) return null;

  const on = enabled && !!track;
  const Icon = on ? Captions : CaptionsOff;
  const openSettings = onOpenSettings ?? (() => setSettingsOpen(true));

  return (
    <>
      <DropdownMenu key={dropdownPortalContainer}>
        <DropdownMenuTrigger
          testID="player-cc-button"
          accessibilityRole="button"
          accessibilityLabel={t("player-captions-menu")}
          accessibilityState={{ checked: on }}
        >
          <Icon size={size} color={theme.colors.foreground} />
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
                onValueChange={(id: string) =>
                  setTrack(tracks.find((tr) => tr.id === id) ?? null)
                }
              >
                <DropdownMenuRadioItem value={OFF} testID="player-cc-track-off">
                  <Text>{t("player-captions-off")}</Text>
                </DropdownMenuRadioItem>
                {tracks.map((tr) => (
                  <DropdownMenuRadioItem
                    key={tr.id}
                    value={tr.id}
                    testID={`player-cc-track-${tr.id}`}
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
