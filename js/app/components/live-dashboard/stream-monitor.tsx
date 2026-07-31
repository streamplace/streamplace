import {
  IconButton,
  Player,
  PlayerUI,
  Text,
  useLivestream,
  useLivestreamStore,
  usePlayerStore,
  useSegmentTiming,
  useTheme,
  zero,
} from "@streamplace/components";
import {
  borderAlphas,
  fontFamilies,
  scrims,
  statusColors,
  surfaces,
  textAlphas,
} from "@streamplace/components/src/lib/theme/tokens";
import { DesktopUi } from "components/mobile/desktop-ui";
import { OfflineCounter } from "components/mobile/offline-counter";
import { Image } from "expo-image";
import { Eye, EyeOff, Signal, Wifi, WifiOff } from "lucide-react-native";
import { useEffect, useState } from "react";
import { View } from "react-native";
import { useLiveUser } from "../../hooks/useLiveUser";
import StreamScreen from "./live-selector";

const { flex, bg, r, borders, layout, p, text, w, h, mt } = zero;

interface StreamMonitorProps {
  userProfile?: any;
  isLive?: boolean;
  videoRef?: any;
}

function PreviewOverlay() {
  const { theme } = useTheme();
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
      <View style={{ margin: 16, gap: 8, alignItems: "flex-start" }}>
        {/* Refined status pill — matches the connection HUD grammar. Indigo
            (accent, not live-red) says "previewing, ready" rather than "on air". */}
        <View
          style={{
            flexDirection: "row",
            alignItems: "center",
            gap: 8,
            paddingHorizontal: 12,
            paddingVertical: 7,
            backgroundColor: scrims.dark,
            borderRadius: 999,
            borderWidth: 1,
            borderColor: borderAlphas.dark.strong,
          }}
        >
          <View
            style={{
              width: 7,
              height: 7,
              borderRadius: 999,
              backgroundColor: theme.colors.primary,
            }}
          />
          <Text
            style={{
              color: "white",
              fontSize: 12,
              fontFamily: fontFamilies.semiBold,
              fontWeight: "600",
              letterSpacing: 0.5,
            }}
          >
            PREVIEW
          </Text>
        </View>
        <View
          style={{
            backgroundColor: scrims.dark,
            borderRadius: 8,
            borderWidth: 1,
            borderColor: borderAlphas.dark.strong,
            paddingHorizontal: 12,
            paddingVertical: 8,
            maxWidth: 340,
          }}
        >
          <Text
            style={{
              color: "white",
              fontSize: 14,
              fontWeight: "600",
              lineHeight: 19,
            }}
          >
            Only you can see this preview.
          </Text>
          <Text
            style={{
              color: textAlphas.dark[2],
              fontSize: 12.5,
              fontWeight: "500",
              lineHeight: 17,
              marginTop: 2,
            }}
          >
            Press “Start Livestream” to go live for everyone.
          </Text>
        </View>
      </View>
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
    if (!ls) return <Wifi size={16} color={textAlphas.dark[2]} />;

    switch (segmentTiming.connectionQuality) {
      case "good":
        return <Wifi size={16} color={statusColors.dark.success} />;
      case "degraded":
        return <Signal size={16} color={statusColors.dark.warning} />;
      case "poor":
        return <WifiOff size={16} color={statusColors.dark.danger} />;
      default:
        return <WifiOff size={16} color={textAlphas.dark[3]} />;
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

  const getStatusDotColor = () => {
    if (!isLive) return textAlphas.dark[3];
    if (!ls) return textAlphas.dark[2];
    switch (segmentTiming.connectionQuality) {
      case "good":
        return statusColors.dark.success;
      case "degraded":
        return statusColors.dark.warning;
      case "poor":
        return statusColors.dark.danger;
      default:
        return textAlphas.dark[3];
    }
  };

  const getStreamTitle = () => {
    if (!ls) {
      return (
        <Text
          style={[text.white, { fontSize: 14, fontWeight: "600" }]}
          numberOfLines={1}
          ellipsizeMode="tail"
        >
          {userProfile?.handle || userProfile?.displayName || "Not live"}
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
        { backgroundColor: surfaces.dark[1] },
        r.lg,
        borders.width.thin,
        { borderColor: borderAlphas.dark.strong },
        layout.flex.column,
        { overflow: "hidden" },
      ]}
    >
      <View
        style={[
          flex.values[1],
          layout.flex.center,
          { backgroundColor: surfaces.dark[1] },
        ]}
      >
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
                  backgroundColor: scrims.dark,
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
          { borderTopColor: borderAlphas.dark.strong },
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
              layout.flex.alignCenter,
              { gap: 8, flexShrink: 0 },
            ]}
          >
            {isLive && userProfile && (
              <IconButton
                size="sm"
                accessibilityLabel={
                  isStreamVisible ? "Hide preview" : "Show preview"
                }
                onPress={() => setIsStreamVisible(!isStreamVisible)}
              >
                {isStreamVisible ? (
                  <EyeOff size={16} color={textAlphas.dark[3]} />
                ) : (
                  <Eye size={16} color={textAlphas.dark[3]} />
                )}
              </IconButton>
            )}
            <View
              style={[layout.flex.row, layout.flex.alignCenter, { gap: 6 }]}
            >
              <View
                style={[
                  w[3],
                  h[3],
                  r.full,
                  { backgroundColor: getStatusDotColor() },
                ]}
              />
              <Text
                style={{
                  color: textAlphas.dark[2],
                  fontSize: 12,
                  fontWeight: "600",
                  letterSpacing: 0.4,
                }}
              >
                {getStreamStatus()}
              </Text>
            </View>
          </View>
        </View>
      </View>
    </View>
  );
}
