import { useNavigation } from "@react-navigation/native";
import {
  borders,
  Button,
  Dashboard,
  useLivestreamStore,
  usePlayerStore,
  useProfile,
  useTheme,
  zero,
} from "@streamplace/components";
import { surfaces, borderAlphas } from "@streamplace/components/src/lib/theme/tokens";
import {
  ProblemsWrapper,
  ProblemsWrapperRef,
} from "@streamplace/components/src/components/dashboard/problems";
import { EmojiPicker } from "components/emoji-picker/emoji-picker";
import { ArrowRight } from "lucide-react-native";
import { useEffect, useMemo, useRef, useState } from "react";
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
        setScreenHeight(window.innerHeight);
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
  const chat = useLivestreamStore((x) => x.chat);
  const ingestConnectionState = usePlayerStore((x) => x.ingestConnectionState);
  const emojiData = useEmojiData();

  // Calculate derived values
  const isConnected = ingestConnectionState === "connected";

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
    // Desktop layout (>= 1200px): a broadcast console. The frame is locked to
    // the viewport (no page scroll) and every panel flexes to fill it like
    // puzzle pieces; the inherently-long panels (chat, settings) scroll
    // internally. `minHeight: 0` is threaded down the flex chain so children
    // can shrink below their content and hand overflow to their own scrollers.
    return (
      <ProblemsWrapper ref={problemsRef}>
        <View
          style={[
            flex.values[1],
            { backgroundColor: surfaces.dark[0], overflow: "hidden" },
            gap.all[4],
            p[4],
          ]}
        >
          <View
            style={[
              layout.flex.row,
              gap.all[4],
              flex.values[1],
              { minHeight: 0 },
            ]}
          >
            {/* Left: video (hero) over stream health */}
            <View style={[flex.values[4], gap.all[4], { minHeight: 0 }]}>
              <View
                style={[
                  flex.values[3],
                  layout.flex.row,
                  gap.all[4],
                  { minHeight: 0 },
                ]}
              >
                <StreamMonitor
                  isLive={isLive}
                  userProfile={profile}
                  videoRef={videoRef}
                />
              </View>

              <View
                style={[
                  flex.values[2],
                  layout.flex.row,
                  gap.all[4],
                  { minHeight: 0 },
                ]}
              >
                <Dashboard.InformationWidget
                  onShowProblems={() => problemsRef.current?.setDismiss(false)}
                />
              </View>
            </View>

            {/* Middle: multistream over chat (chat fills + scrolls) */}
            <View
              style={[
                flex.values[2],
                layout.flex.column,
                gap.all[4],
                { maxWidth: 600, minHeight: 0 },
              ]}
            >
              <MultistreamStatus />
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

            {/* Right: stream settings (header + scroll body + sticky footer) */}
            <View
              style={[
                flex.values[2],
                layout.flex.column,
                gap.all[4],
                { maxWidth: 600, minHeight: 0 },
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
        <ScrollView
          contentContainerStyle={{ paddingBottom: insets.bottom }}
          style={[flex.values[1], { backgroundColor: surfaces.dark[0] }]}
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
