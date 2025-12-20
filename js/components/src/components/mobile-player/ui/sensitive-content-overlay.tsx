import { useEffect, useMemo, useState } from "react";
import { View } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import { runOnJS } from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";
import { BLUR_LABELS, BLUR_WARNINGS } from "../../../lib/metadata-constants";
import { layout } from "../../../lib/theme/atoms";
import { useLivestreamStore } from "../../../livestream-store";
import { usePlayerStore } from "../../../player-store";
import { useSetMuted, useStreamplaceStore } from "../../../streamplace-store";
import { Button, Text } from "../../ui";

export function SensitiveContentOverlay() {
  const livestream = useLivestreamStore((x) => x.livestream);
  const acknowledged = usePlayerStore((x) => x.sensitiveContentAcknowledged);
  const setAcknowledged = usePlayerStore(
    (x) => x.setSensitiveContentAcknowledged,
  );
  const setMuted = useSetMuted();
  const isMuted = useStreamplaceStore((x) => x.muted);
  const [wasMutedByOverlay, setWasMutedByOverlay] = useState(false);

  const shouldShow = useMemo(() => {
    if (acknowledged) return false;
    const hasBlurLabel = livestream?.labels?.some((l) => BLUR_LABELS[l.val]);
    const hasBlurWarning = livestream?.contentWarnings?.warnings?.some(
      (w) => BLUR_WARNINGS[w],
    );

    if (hasBlurLabel || hasBlurWarning) {
      return true;
    }
    return false;
  }, [livestream, acknowledged]);

  useEffect(() => {
    if (shouldShow && !isMuted) {
      setMuted(true);
      setWasMutedByOverlay(true);
    }
  }, [shouldShow, isMuted, setMuted]);

  const handlePress = () => {
    setAcknowledged(true);
    if (wasMutedByOverlay) {
      setMuted(false);
    }
  };

  const tapGesture = Gesture.Tap().onEnd(() => {
    runOnJS(handlePress)();
  });

  if (!shouldShow) return null;

  return (
    <View
      style={[
        {
          position: "absolute",
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          backgroundColor: "rgba(0,0,0,0.95)",
          padding: 20,
        },
        layout.flex.center,
        { zIndex: 99999 },
      ]}
      pointerEvents="box-none"
    >
      <SafeAreaView
        style={[layout.flex.center, { width: "100%" }]}
        pointerEvents="box-none"
      >
        <Text
          weight="bold"
          size="2xl"
          style={{ color: "white", marginBottom: 12, textAlign: "center" }}
        >
          Sensitive Content
        </Text>
        <Text
          style={{
            color: "rgba(255,255,255,0.8)",
            marginBottom: 24,
            textAlign: "center",
            maxWidth: 300,
          }}
        >
          This stream may contain content that is not suitable for all
          audiences.
        </Text>
        <View style={[layout.flex.center]}>
          <GestureDetector gesture={tapGesture}>
            <View>
              <Button
                width="min"
                onPress={() => {
                  handlePress();
                }}
              >
                View stream
              </Button>
            </View>
          </GestureDetector>
        </View>
      </SafeAreaView>
    </View>
  );
}
