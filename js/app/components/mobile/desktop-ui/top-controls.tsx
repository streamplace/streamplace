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
  useTitle,
  zero,
} from "@streamplace/components";
import { Image } from "expo-image";
import { ChevronLeft, MessageSquare, SwitchCamera } from "lucide-react-native";
import {
  Linking,
  Platform,
  Pressable,
  useWindowDimensions,
} from "react-native";
import { useStore } from "store";
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
  const profile = useAuthor();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const { doSetIngestCamera } = useCameraToggle();
  const avatar = useAvatar();
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
                <Image
                  source={
                    avatar
                      ? { uri: avatar }
                      : require("assets/images/goose.png")
                  }
                  style={[
                    {
                      width: 40,
                      height: 40,
                      borderRadius: 20,
                      backgroundColor: colors.gray[800],
                    },
                    borders.width.thin,
                    borders.color.gray[700],
                  ]}
                />

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
          {!embedded && !offline && <LiveBubble />}
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
                  backgroundColor: "rgba(75,75,75, 0.65)",
                },
              ]}
            >
              {!isSmallScreen && <Text size="lg">Powered by</Text>}
              <Image
                source={require("assets/images/cube_small.png")}
                style={{
                  width: 24,
                  height: 24,
                }}
              />
              {!isTinyScreen && <Text size="lg">Streamplace</Text>}
            </Pressable>
          )}
          {isActivelyLive && (
            <>
              <PlayerUI.Viewers />

              <PlayerUI.ClipButton onLoginRequired={openLoginModal} />

              <Pressable onPress={onToggleChat} style={[p[2], r[1]]}>
                <MessageSquare
                  size={20}
                  color={isChatOpen ? colors.primary[500] : colors.white}
                />
              </Pressable>
            </>
          )}
          {ingest !== null && (
            <Pressable onPress={doSetIngestCamera} style={[p[2], r[1]]}>
              <SwitchCamera size={24} color={colors.gray[200]} />
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
