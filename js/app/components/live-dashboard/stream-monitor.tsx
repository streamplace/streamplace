import {
  Player,
  useLivestream,
  useLivestreamStore,
  usePlayerStore,
  zero,
} from "@streamplace/components";
import { Eye, EyeOff, Signal, Wifi, WifiOff } from "@tamagui/lucide-icons";
import { useEffect, useState } from "react";
import { Image, Text, TouchableOpacity, View } from "react-native";
import { useLiveUser } from "../../hooks/useLiveUser";
import { useSegmentTiming } from "../../hooks/useSegmentTiming";
import StreamScreen from "./live-selector";

const { flex, bg, r, borders, layout, p, text, w, h } = zero;

interface StreamMonitorProps {
  userProfile?: any;
  isLive?: boolean;
  videoRef?: any;
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
  const ls = useLivestream();
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
  return (
    <View
      style={[
        flex.values[2],
        bg.gray[800],
        r[3],
        borders.width.thin,
        borders.color.gray[700],
        layout.flex.column,
      ]}
    >
      <View style={[flex.values[1], layout.flex.center, bg.gray[900]]}>
        {isLive && userProfile ? (
          isStreamVisible ? (
            <Player src={userProfile.did} name={userProfile.handle} />
          ) : (
            <View
              style={[
                layout.flex.center,
                { position: "relative", width: "100%", height: "100%" },
              ]}
            >
              <Image
                source={{ uri: currentThumbnail || thumbnails }}
                style={{
                  width: "100%",
                  height: "100%",
                  resizeMode: "contain",
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
          borders.bottom.width.thin,
          borders.bottom.color.gray[700],
        ]}
      >
        <View style={[layout.flex.row, layout.flex.alignCenter, { gap: 12 }]}>
          <Text style={[text.white, { fontSize: 18, fontWeight: "600" }]}>
            {ls?.record.title || "Stream Title"}
          </Text>
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
          <View style={[w[2], h[2], r[1], bg[getConnectionColor()][500]]} />
          <Text style={[text.gray[400], { fontSize: 14 }]}>
            {isLive ? "LIVE" : "OFFLINE"}
          </Text>
          {isLive && segmentTiming.timeBetweenSegments && (
            <Text style={[text.gray[500], { fontSize: 12 }]}>
              {Math.round(segmentTiming.timeBetweenSegments)}ms
            </Text>
          )}
        </View>
      </View>
    </View>
  );
}
