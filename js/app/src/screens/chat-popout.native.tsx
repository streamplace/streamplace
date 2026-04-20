import { useNavigation } from "@react-navigation/native";
import {
  Chat,
  ChatBox,
  LivestreamProvider,
  PlayerProvider,
  StreamNotificationProvider,
  Text,
  useKeyboard,
  useLivestreamInfo,
  usePlayerStore,
  useSegment,
  zero,
} from "@streamplace/components";
import LiveDot from "components/home/live-dot";
import { ArrowLeft } from "lucide-react-native";
import { useCallback, useEffect, useRef } from "react";
import { KeyboardAvoidingView, Pressable, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useUserProfile } from "store/hooks";
import { useEmojiData } from "utils/emoji";

export default function PopoutChat({ route }) {
  const user = route.params?.user;
  if (typeof user !== "string") {
    return (
      <View
        style={[
          zero.flex.values[1],
          zero.layout.flex.justify.center,
          zero.layout.flex.align.center,
        ]}
      >
        <Text style={[zero.text.white]}>No user specified</Text>
      </View>
    );
  }

  return (
    <LivestreamProvider src={user}>
      <PlayerProvider>
        <PopoutChatInner user={user} />
      </PlayerProvider>
    </LivestreamProvider>
  );
}

export function PopoutChatInner({ user }: { user: string }) {
  const setSrc = usePlayerStore((x) => x.setSrc);
  const profile = useUserProfile();
  const navigation = useNavigation();
  const { ingest, profile: streamProfile } = useLivestreamInfo();
  const status = usePlayerStore((x) => x.status);
  const seg = useSegment();
  const emojiData = useEmojiData();

  const segmentReceivedTimeRef = useRef<number | null>(null);
  const lastSegmentIdRef = useRef<string | null>(null);

  // Track when we receive a new segment
  useEffect(() => {
    if (seg && seg.id !== lastSegmentIdRef.current) {
      segmentReceivedTimeRef.current = Date.now();
      lastSegmentIdRef.current = seg.id;
    }
  }, [seg?.id]);

  const getLatency = useCallback((): string => {
    if (!segmentReceivedTimeRef.current) return "";
    const secondsAgo = Math.floor(Date.now() - segmentReceivedTimeRef.current);
    const isThreeDigits = secondsAgo >= 100 && secondsAgo < 1000;
    if (isThreeDigits) {
      return `0${secondsAgo}`;
    }
    return `${secondsAgo}`;
  }, [seg?.id]); // depend on seg.id to update when new segments arrive

  useEffect(() => {
    setSrc(user);
  }, [user]);

  const safe = useSafeAreaInsets();
  const kb = useKeyboard();

  // if a seg exists and it started less than 10 seconds ago, consider it live
  const isLive = seg && Date.now() - new Date(seg.startTime).getTime() < 10000;

  return (
    <KeyboardAvoidingView
      style={[
        {
          position: "relative",
          marginTop: safe.top,
          marginBottom: safe.bottom + kb.keyboardHeight,
          marginLeft: safe.left,
          marginRight: safe.right,
        },
        zero.flex.values[1],
        zero.m[2],
      ]}
    >
      <View style={[zero.flex.values[1]]}>
        <View
          style={[
            zero.layout.flex.row,
            zero.layout.flex.align.center,
            zero.gap.all[4],
            zero.px[4],
          ]}
        >
          <Pressable
            onPress={() => navigation.goBack()}
            style={[
              zero.layout.flex.row,
              zero.layout.flex.align.center,
              zero.p[3],
              { borderRadius: zero.borderRadius.xl },
            ]}
          >
            <ArrowLeft size={20} color="white" />
          </Pressable>
          <View
            style={[
              zero.flex.values[1],
              zero.layout.flex.row,
              zero.layout.flex.justify.between,
              zero.layout.flex.align.center,
            ]}
          >
            <View
              style={[
                zero.flex.values[1],
                zero.layout.flex.row,
                zero.layout.flex.justify.start,
                zero.layout.flex.align.center,
                { marginLeft: -20 },
              ]}
            >
              {isLive ? (
                <LiveDot />
              ) : (
                <View
                  style={[
                    zero.w[3],
                    zero.h[3],
                    zero.m[2],
                    zero.bg.gray[700],
                    { borderRadius: zero.borderRadius.full },
                  ]}
                />
              )}
              <Text style={[zero.text.white]}>
                {streamProfile?.handle || user}
              </Text>
            </View>
            <View
              style={[
                zero.layout.flex.row,
                zero.layout.flex.align.center,
                zero.gap[3],
              ]}
            >
              <Text style={[zero.text.white]}>
                {isLive ? "Live" : "Offline"}
              </Text>
              {isLive && (
                <Text style={[zero.ml[3], zero.typography.mono.lg]}>
                  {getLatency()}ms
                </Text>
              )}
            </View>
          </View>
        </View>
        <View style={[zero.flex.values[1], zero.p[4]]}>
          <StreamNotificationProvider position="top">
            <Chat />
          </StreamNotificationProvider>
          {profile && <ChatBox emojiData={emojiData} isPopout={true} />}
        </View>
      </View>
    </KeyboardAvoidingView>
  );
}
