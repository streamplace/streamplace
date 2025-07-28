import { useNavigation } from "@react-navigation/native";
import {
  PlayerUI,
  Text,
  View,
  useAvatars,
  useCameraToggle,
  useLivestreamInfo,
  zero,
} from "@streamplace/components";
import { ChevronLeft, MessageSquare, SwitchCamera } from "lucide-react-native";
import { Image, Platform, Pressable } from "react-native";
import { LiveBubble } from "./live-bubble";

const { borders, colors, gap, layout, p, r, text } = zero;

interface TopControlBarProps {
  offline: boolean;
  isActivelyLive: boolean;
  ingest: string | null;
  isChatOpen: boolean;
  onToggleChat: () => void;
  safeAreaInsets: { top: number };
}

export function TopControlBar({
  offline,
  isActivelyLive,
  ingest,
  isChatOpen,
  onToggleChat,
  safeAreaInsets,
}: TopControlBarProps) {
  const navigation = useNavigation();
  const { profile } = useLivestreamInfo();
  const { doSetIngestCamera } = useCameraToggle();
  const avatars = useAvatars(profile?.did ? [profile?.did] : []);

  return (
    <View
      style={[
        layout.flex.row,
        layout.flex.spaceBetween,
        layout.flex.alignCenter,
      ]}
    >
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}>
        {Platform.OS !== "web" && (
          <Pressable
            onPress={() => {
              navigation.canGoBack()
                ? navigation.goBack()
                : navigation.navigate("Home", { screen: "StreamList" });
            }}
            style={[p[2], r[1]]}
          >
            <ChevronLeft color="white" size={24} />
          </Pressable>
        )}
        <Image
          source={
            profile?.did
              ? { uri: avatars[profile?.did]?.avatar }
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
          <Text style={[text.white, { fontSize: 16, fontWeight: "600" }]}>
            {profile?.handle}
          </Text>
          {!offline && <LiveBubble />}
        </View>
      </View>

      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}>
        {isActivelyLive && (
          <>
            <PlayerUI.Viewers />

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
  );
}
