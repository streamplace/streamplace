import {
  Player,
  PlayerUI,
  Text,
  useLivestream,
  useLivestreamStore,
  usePlayerStore,
  useSegmentTiming,
  zero,
} from "@streamplace/components";
import { DesktopUi } from "components/mobile/desktop-ui";
import { OfflineCounter } from "components/mobile/offline-counter";
import { Image } from "expo-image";
import { Eye, EyeOff, Signal, Wifi, WifiOff } from "lucide-react-native";
import { useEffect, useState } from "react";
import { TouchableOpacity, View } from "react-native";
import Animated from "react-native-reanimated";
import { useLiveUser } from "../../hooks/useLiveUser";
import StreamScreen from "./live-selector";

const { flex, bg, r, borders, layout, p, text, w, h, mt } = zero;

interface StreamMonitorProps {
  userProfile?: any;
  isLive?: boolean;
  videoRef?: any;
}

function PreviewOverlay() {
  // const opacity = useSharedValue(1);

  // useEffect(() => {
  //   opacity.value = withRepeat(withTiming(0.8, { duration: 1500 }), -1, true);
  // }, [opacity]);

  // const animatedStyle = useAnimatedStyle(() => ({
  //   opacity: opacity.value,
  // }));

  return (
    <View
      style={{
        position: "absolute",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        justifyContent: "flex-start",
        alignItems: "flex-start",
        pointerEvents: "none",
      }}
    >
      <Animated.Text
        style={[
          // animatedStyle,
          {
            paddingLeft: 16,
            paddingTop: 16,
            fontSize: 32,
            fontWeight: "800",
            color: "white",
            letterSpacing: 4,
            textShadowColor: "rgba(0, 0, 0, 0.9)",
            textShadowOffset: { width: 0, height: 2 },
            textShadowRadius: 8,
          },
        ]}
      >
        PREVIEW (NOT LIVE)
      </Animated.Text>
    </View>
  );
}

export default function StreamMonitor({
  userProfile: propUserProfile,
  isLive: propIsLive,
  videoRef,
}: StreamMonitorProps) {
  // Get data from hooks - use props as fallback if provided
  const isUserLive = useLiveUser();
  const profile = useLivestreamStore((x) => x.profile);
  const ingestConnectionState = usePlayerStore((x) => x.ingestConnectionState);
  let ls = useLivestream();
  const segmentTiming = useSegmentTiming();

  // Use hook data primarily, fallback to props
  const isLive = propIsLive ?? isUserLive;
  const userProfile = propUserProfile ?? profile;

  // State for hiding/showing stream and thumbnail rotation
  const [isStreamVisible, setIsStreamVisible] = useState(true);
  const [currentThumbnail, setCurrentThumbnail] = useState<null | string>(null);

  // Mock thumbnails - in a real implementation, these would come from your stream service
  const thumbnails = "/api/playback/" + profile?.did + "/stream.jpg";

  // Rotate thumbnails every 30 seconds when stream is hidden
  useEffect(() => {
    if (!isStreamVisible && isLive) {
      const interval = setInterval(() => {
        setCurrentThumbnail(thumbnails + "?ts=" + String(Date.now()));
      }, 30000); // 30 seconds

      return () => clearInterval(interval);
    }
  }, [isStreamVisible, isLive, thumbnails.length]);

  // Connection quality indicator
  const getConnectionIcon = () => {
    if (!isLive) return null;
    if (!ls) return <Wifi size={16} color="#3b82f6" />;

    switch (segmentTiming.connectionQuality) {
      case "good":
        return <Wifi size={16} color="#10b981" />;
      case "degraded":
        return <Signal size={16} color="#f59e0b" />;
      case "poor":
        return <WifiOff size={16} color="#ef4444" />;
      default:
        return <WifiOff size={16} color="#6b7280" />;
    }
  };

  const getConnectionColor = () => {
    if (!isLive) return "red";
    if (!ls) return "blue";

    switch (segmentTiming.connectionQuality) {
      case "good":
        return "green";
      case "degraded":
        return "yellow";
      case "poor":
        return "red";
      default:
        return "red";
    }
  };

  const getStreamStatus = () => {
    if (!isLive) return "OFFLINE";
    if (!ls) return "NOT LIVE";
    return "LIVE";
  };

  const getStreamTitle = () => {
    if (!ls) {
      return (
        <Text
          style={[
            text.white,
            { fontSize: 14, fontWeight: "400", fontStyle: "italic" },
          ]}
          numberOfLines={1}
          ellipsizeMode="tail"
        >
          Stream not live yet. Press "Announce Livestream" to start!
        </Text>
      );
    }
    return (
      <Text
        style={[text.white, { fontSize: 18, fontWeight: "600" }]}
        numberOfLines={1}
        ellipsizeMode="tail"
      >
        {ls?.record.title || "Stream Title"}
      </Text>
    );
  };

  return (
    <View
      style={[
        flex.values[2],
        bg.gray[800],
        r.lg,
        bg.neutral[900],
        borders.width.thin,
        borders.color.neutral[700],
        layout.flex.column,
        { overflow: "hidden" },
      ]}
    >
      <View style={[flex.values[1], layout.flex.center, bg.neutral[900]]}>
        {isLive && userProfile ? (
          isStreamVisible ? (
            <View
              style={{ position: "relative", width: "100%", height: "100%" }}
            >
              <Player
                src={userProfile.did}
                name={userProfile.handle}
                muted={true}
              >
                <DesktopUi />
                <PlayerUI.ViewerLoadingOverlay />
                <OfflineCounter isMobile={true} />
              </Player>
              {!ls && <PreviewOverlay />}
            </View>
          ) : (
            <View
              style={[
                layout.flex.center,
                { position: "relative", width: "100%", height: "100%" },
              ]}
            >
              <Image
                source={{ uri: currentThumbnail || thumbnails }}
                contentFit="contain"
                style={{
                  width: "100%",
                  height: "100%",
                }}
              />
              <View
                style={{
                  position: "absolute",
                  top: 12,
                  left: 12,
                  backgroundColor: "rgba(0, 0, 0, 0.9)",
                  paddingHorizontal: 8,
                  paddingVertical: 4,
                  borderRadius: 4,
                }}
              >
                <Text style={[text.white, { fontSize: 12 }]}>
                  Thumbnail Preview
                </Text>
              </View>
            </View>
          )
        ) : (
          <StreamScreen route={profile?.did} />
        )}
      </View>
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          p[4],
          borders.top.width.thin,
          borders.top.color.gray[700],
        ]}
      >
        <View
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            { flex: 1, minWidth: 0, gap: 12 },
          ]}
        >
          <View style={{ flex: 1, minWidth: 0 }}>{getStreamTitle()}</View>
          <View
            style={[
              layout.flex.row,
              layout.flex.justify.end,
              layout.flex.align.start,
              { gap: 8, flexShrink: 0 },
            ]}
          >
            {isLive && userProfile && (
              <TouchableOpacity
                onPress={() => setIsStreamVisible(!isStreamVisible)}
                style={{
                  padding: 4,
                  borderRadius: 4,
                  backgroundColor: "rgba(255, 255, 255, 0.1)",
                }}
              >
                {isStreamVisible ? (
                  <EyeOff size={16} color="#9ca3af" />
                ) : (
                  <Eye size={16} color="#9ca3af" />
                )}
              </TouchableOpacity>
            )}
            <View
              style={[
                w[3],
                h[3],
                r.full,
                { marginTop: 3 },
                bg[getConnectionColor()][500],
              ]}
            />
            <Text style={[text.gray[400], { fontSize: 14 }]}>
              {getStreamStatus()}
            </Text>
          </View>
        </View>
      </View>
    </View>
  );
}
