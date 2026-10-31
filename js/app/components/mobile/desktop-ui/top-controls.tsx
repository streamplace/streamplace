import { useNavigation } from "@react-navigation/native";
import {
  ContentWarningBadge,
  PlayerUI,
  Text,
  View,
  useAuthor,
  useAvatar,
  useCameraToggle,
  useLivestreamStore,
  usePlayerStore,
  useTheme,
  useTitle,
  zero,
} from "@streamplace/components";
import {
  borderRadius,
  scrims,
  spacing,
  textAlphas,
} from "@streamplace/components/src/lib/theme/tokens";
import { formatHandleWithAt } from "@streamplace/core";
import { LogoTile } from "components/brand/logo";
import { Image } from "expo-image";
import { ChevronLeft, MessageSquare, SwitchCamera } from "lucide-react-native";
import {
  Linking,
  Platform,
  Pressable,
  useWindowDimensions,
} from "react-native";
import { convertNavigationParams } from "../../../src/navigation-helper";
import { LiveBubble } from "./live-bubble";

const { borders, colors, gap, layout, p, px, py, r, text } = zero;

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
  const fullscreen = usePlayerStore((state) => state.fullscreen);
  const showIdentity = embedded || (Platform.OS === "web" && fullscreen);
  const { width } = useWindowDimensions();
  const isTinyScreen = width < 450;
  const isSmallScreen = width < 600;

  const title = useTitle();

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
        <View
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            gap.all[3],
            { flex: 1, minWidth: 0 },
          ]}
        >
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
          {showIdentity && (
            <View style={[gap.all[2], { flex: 1, minWidth: 0 }]}>
              <View
                style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
              >
                {embedded && (
                  <Image
                    source={
                      avatar
                        ? { uri: avatar }
                        : require("assets/images/goose.png")
                    }
                    style={[
                      {
                        width: spacing[8],
                        height: spacing[8],
                        borderRadius: borderRadius.full,
                        backgroundColor: theme.colors.surface2,
                        borderColor: theme.colors.border,
                      },
                      borders.width.thin,
                    ]}
                  />
                )}

                <View style={[layout.flex.column, { flex: 1, minWidth: 0 }]}>
                  <Text
                    numberOfLines={1}
                    size="sm"
                    weight="medium"
                    style={{ color: colors.white }}
                  >
                    {title}
                  </Text>
                  {profile && (
                    <Text
                      numberOfLines={1}
                      size="xs"
                      style={{ color: textAlphas.dark[2] }}
                    >
                      {formatHandleWithAt(profile)}
                    </Text>
                  )}
                </View>
              </View>
            </View>
          )}

          {!showIdentity && <ContentWarningBadge warnings={contentWarnings} />}
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
              <LogoTile size={24} />
              {!isTinyScreen && <Text size="lg">Streamplace</Text>}
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
      {showIdentity && contentWarnings.length > 0 && (
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
