import { useNavigation } from "@react-navigation/native";
import {
  Avatar,
  ContentWarningBadge,
  PlayerUI,
  Text,
  View,
  useAuthor,
  useAvatar,
  useBrandingAsset,
  useCameraToggle,
  useLivestreamStore,
  useTheme,
  useTitle,
  zero,
} from "@streamplace/components";
import {
  scrims,
  textAlphas,
} from "@streamplace/components/src/lib/theme/tokens";
import { BRAND, LogoMark } from "components/brand/logo";
import { ChevronLeft, MessageSquare, SwitchCamera } from "lucide-react-native";
import {
  Linking,
  Platform,
  Pressable,
  useWindowDimensions,
} from "react-native";
import { convertNavigationParams } from "../../../src/navigation-helper";
import { LiveBubble } from "./live-bubble";

const { colors, gap, layout, p, px, py, r, text } = zero;

interface TopControlBarProps {
  offline: boolean;
  isActivelyLive: boolean;
  ingest: string | null;
  isChatOpen: boolean;
  onToggleChat: () => void;
  embedded?: boolean;
}

export function TopControlBar({
  offline,
  isActivelyLive,
  ingest,
  isChatOpen,
  onToggleChat,
  embedded = false,
}: TopControlBarProps) {
  const navigation = useNavigation();
  const { theme } = useTheme();
  const profile = useAuthor();
  const { doSetIngestCamera } = useCameraToggle();
  const avatar = useAvatar();
  const { width } = useWindowDimensions();
  const isTinyScreen = width < 450;
  const isSmallScreen = width < 600;

  const title = useTitle();
  // "Powered by" names the operator's brand when one is configured, else ours.
  const nodeTitle = useBrandingAsset("siteTitle")?.data || BRAND.name;

  // Get content warnings from segment
  const segment = useLivestreamStore((x) => x.segment);
  const contentWarnings =
    (segment?.contentWarnings?.warnings as string[]) || [];

  return (
    <View style={[layout.flex.column, gap.all[2]]}>
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
        ]}
      >
        <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}>
          {Platform.OS !== "web" && !embedded && (
            <Pressable
              onPress={() => {
                if (navigation.canGoBack()) {
                  navigation.goBack();
                } else {
                  const params = convertNavigationParams({
                    screen: "HomeMain",
                  });
                  navigation.navigate(params.screen as any, params.params);
                }
              }}
              style={[p[2], r[1]]}
            >
              <ChevronLeft color="white" size={24} />
            </Pressable>
          )}
          {embedded && (
            <View style={[gap.all[2]]}>
              <View
                style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
              >
                <Avatar src={avatar} name={profile?.handle} size={40} />

                <View style={[layout.flex.column]}>
                  <Text weight="semibold">{title}</Text>
                  <Text leading="tight">{profile?.handle}</Text>
                </View>
              </View>
            </View>
          )}

          {!embedded && <ContentWarningBadge warnings={contentWarnings} />}
        </View>

        <View
          style={[
            layout.flex.row,
            layout.flex.align.start,
            layout.flex.justify.start,
            gap.all[3],
          ]}
        >
          {!embedded && !offline && (
            <LiveBubble broadcasting={ingest !== null} />
          )}
          {embedded && Platform.OS === "web" && (
            <Pressable
              onPress={() => {
                const url = window.location.href.replace("/embed/", "/");
                Linking.openURL(url);
              }}
              style={[
                layout.flex.row,
                layout.flex.alignCenter,
                gap.all[2],
                py[2],
                px[3],
                r.xl,
                {
                  backgroundColor: scrims.dark,
                },
              ]}
            >
              {!isSmallScreen && <Text size="lg">Powered by</Text>}
              <LogoMark size={24} color={colors.white} />
              {!isTinyScreen && <Text size="lg">{nodeTitle}</Text>}
            </Pressable>
          )}
          {isActivelyLive && (
            <>
              <PlayerUI.Viewers />

              <Pressable onPress={onToggleChat} style={[p[2], r[1]]}>
                <MessageSquare
                  size={20}
                  color={isChatOpen ? theme.colors.primary : colors.white}
                />
              </Pressable>
            </>
          )}
          {ingest !== null && (
            <Pressable onPress={doSetIngestCamera} style={[p[2], r[1]]}>
              <SwitchCamera size={24} color={textAlphas.dark[1]} />
            </Pressable>
          )}
        </View>
      </View>
      {embedded && (
        <View
          style={[
            layout.flex.row,
            layout.flex.align.start,
            layout.flex.justify.start,
            gap.all[3],
          ]}
        >
          <ContentWarningBadge warnings={contentWarnings} />
        </View>
      )}
    </View>
  );
}
