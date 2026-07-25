import { useNavigation } from "@react-navigation/native";
import {
  borders,
  Button,
  Dashboard,
  useLivestream,
  useLivestreamStore,
  usePlayerStore,
  useProfile,
  useSegment,
  useSegmentTiming,
  useTheme,
  zero,
} from "@streamplace/components";
import {
  ProblemsWrapper,
  ProblemsWrapperRef,
} from "@streamplace/components/src/components/dashboard/problems";
import { EmojiPicker } from "components/emoji-picker/emoji-picker";
import { ArrowRight } from "lucide-react-native";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Dimensions, Platform, ScrollView, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useEmojiData } from "utils/emoji";
import LivestreamPanel from "./livestream-panel";
import MultistreamStatus from "./multistream-status";
import StreamMonitor from "./stream-monitor";

const { flex, p, gap, layout, bg } = zero;

interface BentoGridProps {
  isLive: boolean;
  videoRef: any;
}

export default function BentoGrid({ isLive, videoRef }: BentoGridProps) {
  const navigation = useNavigation();
  const isWeb = Platform.OS === "web";
  const problemsRef = useRef<ProblemsWrapperRef>(null);

  const handleProblemsPress = useCallback(() => {
    problemsRef.current?.setDismiss(false);
  }, []);

  // Screen width state for responsive design
  const [screenWidth, setScreenWidth] = useState(
    isWeb ? window.innerWidth : Dimensions.get("window").width,
  );
  const [screenHeight, setScreenHeight] = useState(
    isWeb ? window.innerHeight : Dimensions.get("window").height,
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

  const insets = useSafeAreaInsets();
  const { theme } = useTheme();

  // Get data from hooks for Dashboard components
  const profile = useProfile();
  const viewers = useLivestreamStore((x) => x.viewers);
  const chat = useLivestreamStore((x) => x.chat);
  const problems = useLivestreamStore((x) => x.problems);
  const segmentTiming = useSegmentTiming();
  const seg = useSegment();
  const ingestConnectionState = usePlayerStore((x) => x.ingestConnectionState);
  const ingestStarted = usePlayerStore((x) => x.ingestStarted);
  const emojiData = useEmojiData();
  const livestream = useLivestream();

  // Calculate derived values
  const isConnected = ingestConnectionState === "connected";

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
    | "offline"
    | "pre-live" => {
    if (!isLive) return "offline";
    if (!livestream) return "pre-live";
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
  }, [isLive, livestream, segmentTiming.connectionQuality]);

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
      <ProblemsWrapper ref={problemsRef}>
        <View style={[flex.values[1], gap.all[4], p[4], bg.black]}>
          <View
            style={[layout.flex.column, { minWidth: isWeb ? 400 : "100%" }]}
          >
            <Dashboard.Header
              isLive={isLive}
              streamTitle={
                profile?.displayName || profile?.handle || "Live Stream"
              }
              uptime={getUptime()}
              bitrate={getBitrate()}
              timeBetweenSegments={segmentTiming.timeBetweenSegments || 0}
              connectionStatus={getConnectionStatus}
              problemsCount={problems.length}
              onProblemsPress={handleProblemsPress}
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
              <View
                style={[
                  borders.bottom.width.thin,
                  borders.bottom.color.neutral[700],
                ]}
              >
                <MultistreamStatus />
              </View>
              <Dashboard.ChatPanel
                isLive={isLive}
                isConnected={isConnected}
                messagesPerMinute={messagesPerMinute}
                emojiData={emojiData}
                emojiPicker={(isOpen, onClose, onSelect) => (
                  <EmojiPicker
                    isOpen={isOpen}
                    onClose={onClose}
                    onSelect={onSelect}
                    customEmoji={[]}
                  />
                )}
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
      </ProblemsWrapper>
    );
  }

  return (
    <ProblemsWrapper ref={problemsRef}>
      <>
        {/* Header always at top */}
        <View style={[p[4]]}>
          <Dashboard.Header
            isLive={isLive}
            streamTitle={
              profile?.displayName || profile?.handle || "Live Stream"
            }
            uptime={getUptime()}
            bitrate={getBitrate()}
            timeBetweenSegments={segmentTiming.timeBetweenSegments || 0}
            connectionStatus={getConnectionStatus}
            problemsCount={problems.length}
            onProblemsPress={handleProblemsPress}
          />
        </View>
        <ScrollView
          contentContainerStyle={{ paddingBottom: insets.bottom }}
          style={[flex.values[1], bg.black]}
        >
          <View style={[gap.all[4], p[4], { paddingTop: 0 }]}>
            {/* Stream Monitor Panel */}
            <View style={[{ height: screenHeight * 0.35 }]}>
              <StreamMonitor
                isLive={isLive}
                userProfile={profile}
                videoRef={videoRef}
              />
            </View>
            <Button
              disabled={!profile}
              onPress={() =>
                navigation.navigate("PopoutChat", { user: profile!.did })
              }
              size="lg"
              rightIcon={<ArrowRight size="18" color={theme.colors.text} />}
            >
              Go to chat
            </Button>
            <View>
              <LivestreamPanel scrollable={false} />
            </View>
          </View>
        </ScrollView>
      </>
    </ProblemsWrapper>
  );
}
