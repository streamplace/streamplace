import { Dashboard, useProfile, useTheme } from "@streamplace/components";
import {
  ProblemsWrapper,
  ProblemsWrapperRef,
} from "@streamplace/components/src/components/dashboard/problems";
import { breakpoints } from "@streamplace/components/src/lib/theme/tokens";
import { EmojiPicker } from "components/emoji-picker/emoji-picker";
import { useRef } from "react";
import { ScrollView, useWindowDimensions, View } from "react-native";
import { useEmojiData } from "utils/emoji";
import CreatorHealth from "./creator-health.web";
import { useCreatorMetrics } from "./creator-metrics-provider.web";
import LivestreamPanel from "./livestream-panel";
import MultistreamStatus from "./multistream-status";
import StreamMonitor from "./stream-monitor";

export default function CreatorDashboard({ videoRef }: { videoRef: any }) {
  const { theme } = useTheme();
  const { width, height } = useWindowDimensions();
  const stacked = width < breakpoints.xl;
  const { status, history } = useCreatorMetrics();
  const profile = useProfile();
  const emojiData = useEmojiData();
  const problemsRef = useRef<ProblemsWrapperRef>(null);
  const messagesPerMinute = history[history.length - 1]?.chatRate ?? null;
  const spacing = theme.spacing[4];

  return (
    <ProblemsWrapper ref={problemsRef}>
      <ScrollView
        style={{ flex: 1, backgroundColor: theme.colors.surface0 }}
        contentContainerStyle={{
          flexGrow: 1,
          ...(stacked ? {} : { height: "100%" }),
          padding: spacing,
        }}
      >
        <View
          style={{
            flex: 1,
            flexDirection: stacked ? "column" : "row",
            gap: spacing,
            minHeight: 0,
          }}
        >
          <View
            style={{
              flex: stacked ? undefined : 4,
              gap: spacing,
              minWidth: 0,
              minHeight: 0,
            }}
          >
            <View
              style={
                stacked
                  ? { height: Math.min(height * 0.45, width * 0.65) }
                  : { flex: 3, minHeight: 0 }
              }
            >
              <StreamMonitor
                status={status}
                userProfile={profile}
                videoRef={videoRef}
              />
            </View>
            <View
              style={
                stacked
                  ? { minHeight: theme.spacing[16] * 4 }
                  : { flex: 2, minHeight: 0 }
              }
            >
              <CreatorHealth
                onShowProblems={() => problemsRef.current?.setDismiss(false)}
              />
            </View>
          </View>
          <View
            style={{
              flex: stacked ? undefined : 2,
              gap: spacing,
              minWidth: 0,
              minHeight: 0,
            }}
          >
            <MultistreamStatus />
            <View
              style={
                stacked
                  ? { height: theme.spacing[16] * 8 }
                  : { flex: 1, minHeight: 0 }
              }
            >
              <Dashboard.ChatPanel
                isLive={status.phase === "live"}
                isConnected={status.incoming !== null}
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
          </View>
          <View
            style={{
              flex: stacked ? undefined : 2,
              minWidth: 0,
              minHeight: 0,
            }}
          >
            <LivestreamPanel status={status} scrollable={!stacked} />
          </View>
        </View>
      </ScrollView>
    </ProblemsWrapper>
  );
}
