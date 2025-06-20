import { useNavigation } from "@react-navigation/native";
import {
  atoms,
  PlayerUI,
  Text,
  Toast,
  useAvatars,
  useCameraToggle,
  useKeyboardSlide,
  useLivestreamInfo,
  usePlayerDimensions,
  useSegmentTiming as useSegmentMetrics,
  View,
} from "@streamplace/components";
import { ChevronLeft, SwitchCamera } from "lucide-react-native";
import { useEffect } from "react";
import { Image, Pressable } from "react-native";
import { ChatPanel } from "./chat";

const { borders, colors, gap, h, layout, position, w } = atoms;

export function MobileUi() {
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
  const { width, height, isPlayerRatioGreater } = usePlayerDimensions();
  const { slideKeyboard } = useKeyboardSlide();
  const { connectionQuality, segmentDeltas, mean, range } = useSegmentMetrics();
  const { doSetIngestCamera } = useCameraToggle();
  const avatars = useAvatars(profile?.did ? [profile?.did] : []);

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
      <View style={[layout.position.absolute, h.percent[100], w.percent[100]]}>
        <View
          style={[
            {
              padding: 6.5,
              backgroundColor: "rgba(0, 0, 0, 0.6)",
              borderRadius: 8,
            },
            layout.position.absolute,
            position.left[1],
            position.top[1],
          ]}
        >
          <View style={[layout.flex.row, layout.flex.center, gap.all[2]]}>
            <Pressable
              onPress={() => {
                navigation.canGoBack()
                  ? navigation.goBack()
                  : navigation.navigate("Home", { screen: "StreamList" });
              }}
            >
              <ChevronLeft />
            </Pressable>
            <Image
              source={
                profile?.did
                  ? { url: avatars[profile?.did]?.avatar }
                  : require("assets/images/goose.png")
              }
              width={32}
              height={32}
              style={[
                {
                  width: 36,
                  height: 36,
                  borderRadius: 999,
                  backgroundColor: "green",
                },
                borders.width.thin,
                borders.color.gray[700],
              ]}
            />
            <Text>{profile?.handle}</Text>
          </View>
        </View>
        <View
          style={[
            {
              padding: 10,
              backgroundColor: "rgba(0, 0, 0, 0.5)",
              borderRadius: 8,
            },
            layout.position.absolute,
            position.right[1],
            position.top[1],
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
      </View>
      {isLive && (
        <View
          style={[
            layout.position.absolute,
            position.top[14],
            position.left[0],
            position.right[0],
            layout.flex.column,
            layout.flex.center,
          ]}
        >
          <PlayerUI.MetricsPanel
            connectionQuality={connectionQuality}
            showMetrics={isLive || isSelfAndNotLive}
            segmentDeltas={segmentDeltas}
            mean={mean || 999}
            range={range || 999}
          />
        </View>
      )}
      {isSelfAndNotLive ? (
        <PlayerUI.InputPanel
          title={title}
          setTitle={setTitle}
          ingestStarting={ingestStarting}
          toggleGoLive={toggleGoLive}
          slideKeyboard={slideKeyboard}
        />
      ) : (
        <ChatPanel
          isPlayerRatioGreater={isPlayerRatioGreater}
          slideKeyboard={slideKeyboard}
        />
      )}
      <PlayerUI.CountdownOverlay
        visible={showCountdown}
        width={width}
        height={height}
        startFrom={3}
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
    </>
  );
}
