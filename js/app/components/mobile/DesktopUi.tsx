import { useNavigation } from "@react-navigation/native";
import {
  PlayerUI,
  Text,
  Toast,
  useAvatars,
  useCameraToggle,
  useLivestreamInfo,
  usePlayerDimensions,
  View,
  zero,
} from "@streamplace/components";
import { ChevronLeft, MessageCircle, SwitchCamera } from "lucide-react-native";
import { useEffect } from "react";
import { Image, Pressable, TouchableWithoutFeedback } from "react-native";
import { useResponsiveLayout } from "./useResponsiveLayout";

const { borders, colors, gap, h, layout, position, w, px, py, r } = zero;

export function DesktopUi({
  showChat,
  setShowChat,
}: {
  showChat: boolean;
  setShowChat: (show: boolean) => void;
}) {
  const navigation = useNavigation();
  const {
    ingest,
    profile,
    title,
    setTitle,
    showCountdown,
    setShowCountdown,
    recordSubmitted,
    setRecordSubmitted,
    ingestStarting,
    setIngestStarting,
    toggleGoLive,
  } = useLivestreamInfo();
  const { width, height } = usePlayerDimensions();
  const { doSetIngestCamera } = useCameraToggle();
  const avatars = useAvatars(profile?.did ? [profile?.did] : []);

  // Desktop layout configuration
  const { shouldShowChatSidePanel, chatPanelWidth, safeAreaInsets } =
    useResponsiveLayout();

  useEffect(() => {
    return () => {
      if (ingestStarting) {
        setIngestStarting(false);
      }
    };
  }, [ingestStarting, setIngestStarting]);

  const isSelfAndNotLive = ingest === "new";
  const isLive = ingest !== null && ingest !== "new";

  return (
    <>
      <TouchableWithoutFeedback>
        <View
          style={[layout.position.absolute, h.percent[100], w.percent[100]]}
        >
          {/* Main UI Overlay */}
          <View
            style={[layout.position.absolute, h.percent[100], w.percent[100]]}
          >
            {/* Top Left - Back Button and Profile */}
            <View
              style={[
                {
                  padding: 8,
                  backgroundColor: "rgba(0, 0, 0, 0.6)",
                },
                r[2],
                layout.position.absolute,
                position.left[2],
                { top: safeAreaInsets.top + 12 },
              ]}
            >
              <View style={[layout.flex.row, layout.flex.center, gap.all[3]]}>
                <Pressable
                  onPress={() => {
                    navigation.canGoBack()
                      ? navigation.goBack()
                      : navigation.navigate("Home", { screen: "StreamList" });
                  }}
                >
                  <ChevronLeft color="white" size={24} />
                </Pressable>
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
                      backgroundColor: "green",
                      borderRadius: 20,
                    },
                    borders.width.thin,
                    borders.color.gray[700],
                  ]}
                />
                <Text
                  style={{ fontSize: 16, fontWeight: "600", color: "white" }}
                >
                  {profile?.handle}
                </Text>
              </View>
            </View>

            {/* Top Center - Viewers and Metrics */}
            {isLive && (
              <View
                style={[
                  layout.position.absolute,
                  { top: safeAreaInsets.top + 12 },
                  position.left[0],
                  position.right[0],
                  layout.flex.column,
                  layout.flex.center,
                ]}
              >
                <View
                  style={[
                    {
                      padding: 12,
                      backgroundColor: "rgba(0, 0, 0, 0.5)",
                    },
                    r[3],
                    layout.flex.row,
                    layout.flex.center,
                    gap.all[4],
                  ]}
                >
                  <PlayerUI.Viewers />
                  <PlayerUI.MetricsPanel showMetrics={isLive} />
                </View>
              </View>
            )}

            {/* Top Right Corner - Context Menu/Camera Toggle */}
            <View
              style={[
                {
                  padding: 10,
                  backgroundColor: "rgba(0, 0, 0, 0.5)",
                },
                r[2],
                layout.position.absolute,
                position.right[2],
                { top: safeAreaInsets.top + 12 },
                gap.all[4],
              ]}
            >
              {ingest === null ? (
                <PlayerUI.ContextMenu />
              ) : (
                <Pressable onPress={doSetIngestCamera}>
                  <SwitchCamera size={32} color={colors.gray[200]} />
                </Pressable>
              )}
            </View>

            {/* Chat Toggle Button - shown when chat is collapsed */}
            {shouldShowChatSidePanel && !showChat && (
              <Pressable
                style={[
                  {
                    padding: 12,
                    backgroundColor: "rgba(0, 0, 0, 0.7)",
                    borderRadius: 8,
                  },
                  layout.position.absolute,
                  position.right[2],
                  { top: safeAreaInsets.top + 70 },
                  layout.flex.row,
                  layout.flex.center,
                  gap.all[2],
                ]}
                onPress={() => setShowChat(true)}
              >
                <MessageCircle size={20} color={colors.gray[200]} />
                <Text style={{ color: "white", fontSize: 14 }}>Show Chat</Text>
              </Pressable>
            )}
          </View>

          {/* Input Panel for self streams */}
          {isSelfAndNotLive && (
            <PlayerUI.InputPanel
              title={title}
              setTitle={setTitle}
              ingestStarting={ingestStarting}
              toggleGoLive={toggleGoLive}
            />
          )}

          <PlayerUI.CountdownOverlay
            visible={showCountdown}
            width={width}
            height={height}
            onDone={() => {
              setShowCountdown(false);
            }}
          />

          <Toast
            open={recordSubmitted}
            onOpenChange={setRecordSubmitted}
            title="You're live!"
            description="We're notifying your followers that you just went live."
            duration={5}
          />
        </View>
      </TouchableWithoutFeedback>
    </>
  );
}
