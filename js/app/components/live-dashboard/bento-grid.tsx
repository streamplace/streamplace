import {
  Dashboard,
  useLivestreamStore,
  usePlayerStore,
  useProfile,
  useSegment,
  useSegmentTiming,
  zero,
} from "@streamplace/components";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Dimensions, Platform, ScrollView, View } from "react-native";
import { useLiveUser } from "../../hooks/useLiveUser";
import LivestreamPanel from "./livestream-panel";
import StreamMonitor from "./stream-monitor";

const { flex, p, gap, layout, bg } = zero;

interface BentoGridProps {
  userProfile: any;
  isLive: boolean;
  videoRef: any;
}

export default function BentoGrid({
  userProfile,
  isLive,
  videoRef,
}: BentoGridProps) {
  const isWeb = Platform.OS === "web";

  // Screen width state for responsive design
  const [screenWidth, setScreenWidth] = useState(
    isWeb ? window.innerWidth : Dimensions.get("window").width,
  );

  useEffect(() => {
    if (isWeb) {
      const handleResize = () => {
        setScreenWidth(window.innerWidth);
      };
      window.addEventListener("resize", handleResize);
      return () => window.removeEventListener("resize", handleResize);
    } else {
      const subscription = Dimensions.addEventListener(
        "change",
        ({ window }) => {
          setScreenWidth(window.width);
        },
      );
      return () => subscription?.remove();
    }
  }, [isWeb]);

  const isDesktop = screenWidth >= 1200;

  // Get data from hooks for Dashboard components
  const profile = useProfile();
  const viewers = useLivestreamStore((x) => x.viewers);
  const chat = useLivestreamStore((x) => x.chat);
  const segmentTiming = useSegmentTiming();
  const seg = useSegment();
  const ingestConnectionState = usePlayerStore((x) => x.ingestConnectionState);
  const ingestStarted = usePlayerStore((x) => x.ingestStarted);
  const userIsLive = useLiveUser();

  // Calculate derived values
  const isConnected = ingestConnectionState === "connected";
  const canModerate = isLive && isConnected;

  // Calculate uptime
  const getUptime = useCallback((): string => {
    if (!ingestStarted || !isLive) return "00:00:00";
    const uptimeMs = Date.now() - ingestStarted;
    const seconds = Math.floor(uptimeMs / 1000);
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;
    return `${hours.toString().padStart(2, "0")}:${minutes.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  }, [ingestStarted, isLive]);

  // Calculate bitrate
  const getBitrate = useCallback((): string => {
    if (!seg?.size || !seg?.duration) return "0 kbps";
    const kbps =
      (seg.size * 8) /
      ((seg.duration || 1000000000) / 1000000000) /
      1000 /
      1000;
    return `${kbps.toFixed(2)} mbps`;
  }, [seg?.size, seg?.duration]);

  // Map connection quality to status
  const getConnectionStatus = useMemo(():
    | "excellent"
    | "good"
    | "poor"
    | "offline" => {
    if (!isLive) return "offline";
    switch (segmentTiming.connectionQuality) {
      case "good":
        return "excellent";
      case "degraded":
        return "good";
      case "poor":
        return "poor";
      default:
        return "offline";
    }
  }, [isLive, segmentTiming.connectionQuality]);

  // Calculate messages per minute
  const messagesPerMinute = useMemo((): number => {
    const now = Date.now();
    const oneMinuteAgo = now - 60 * 1000;
    return (
      chat?.filter((msg) => {
        try {
          const ts = new Date(msg.indexedAt).getTime();
          return ts > oneMinuteAgo;
        } catch (e) {
          return false;
        }
      })?.length || 0
    );
  }, [chat]);

  if (isDesktop) {
    // Desktop layout (>= 1200px) - Original bento grid
    return (
      <View style={[flex.values[1], gap.all[4], p[4], bg.black]}>
        <View style={[layout.flex.column, { minWidth: isWeb ? 400 : "100%" }]}>
          <Dashboard.Header
            isLive={isLive}
            streamTitle={
              profile?.displayName || profile?.handle || "Live Stream"
            }
            viewers={viewers || 0}
            uptime={getUptime()}
            bitrate={getBitrate()}
            timeBetweenSegments={segmentTiming.timeBetweenSegments || 0}
            connectionStatus={getConnectionStatus}
          />
        </View>
        <View style={[flex.values[1], layout.flex.row, gap.all[4]]}>
          <View style={[flex.values[4], gap.all[4]]}>
            <View
              style={[
                flex.values[2],
                layout.flex.row,
                gap.all[4],
                { height: isWeb ? 300 : 200 },
              ]}
            >
              <StreamMonitor
                isLive={isLive}
                userProfile={profile}
                videoRef={videoRef}
              />
            </View>

            <View style={[layout.flex.row, gap.all[4], flex.values[1]]}>
              <Dashboard.InformationWidget />
            </View>
          </View>

          <View
            style={[
              flex.values[2],
              layout.flex.column,
              gap.all[4],
              { maxWidth: isWeb ? 600 : "100%" },
            ]}
          >
            <Dashboard.ChatPanel
              isLive={isLive}
              isConnected={isConnected}
              messagesPerMinute={messagesPerMinute}
              canModerate={canModerate}
            />
          </View>
          <View
            style={[
              flex.values[2],
              layout.flex.column,
              gap.all[4],
              { maxWidth: isWeb ? 600 : "100%" },
            ]}
          >
            <LivestreamPanel />
          </View>
        </View>
      </View>
    );
  }

  return (
    <View style={[flex.values[1], bg.black]}>
      {/* Header always at top */}
      <View style={[p[4]]}>
        <Dashboard.Header
          isLive={isLive}
          streamTitle={profile?.displayName || profile?.handle || "Live Stream"}
          viewers={viewers || 0}
          uptime={getUptime()}
          bitrate={getBitrate()}
          timeBetweenSegments={segmentTiming.timeBetweenSegments || 0}
          connectionStatus={getConnectionStatus}
        />
      </View>

      {/* Vertical scrolling content */}
      <ScrollView
        showsVerticalScrollIndicator={false}
        style={[flex.values[1]]}
        contentContainerStyle={[
          layout.flex.column,
          gap.all[4],
          p[4],
          { paddingTop: 0 },
        ]}
      >
        {/* Stream Monitor Panel */}
        <View style={[layout.flex.column, { minHeight: 512 }]}>
          <StreamMonitor
            isLive={isLive}
            userProfile={profile}
            videoRef={videoRef}
          />
        </View>

        {/* Chat Panel */}
        <View style={[layout.flex.column]}>
          <Dashboard.ChatPanel
            isLive={isLive}
            isConnected={isConnected}
            messagesPerMinute={messagesPerMinute}
            canModerate={canModerate}
          />
        </View>

        {/* Livestream Panel */}
        <View style={[layout.flex.column]}>
          <LivestreamPanel />
        </View>
      </ScrollView>
    </View>
  );
}
