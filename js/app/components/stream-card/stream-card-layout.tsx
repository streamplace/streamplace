import { useNavigation } from "@react-navigation/native";
import {
  Avatar,
  Chat,
  ChatBox,
  KeepAwake,
  Loader,
  Text,
  useLivestreamStore,
  useSocialShell,
  useTheme,
  useToast,
  View,
  zero,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { EmojiPicker } from "components/emoji-picker/emoji-picker";
import { useStreamMeta } from "components/mobile/bottom-metadata";
import { Player } from "components/mobile/player";
import { PlayerProps } from "components/player/props";
import { MessageIcon } from "components/sidebar/social-icons";
import { FullscreenProvider } from "contexts/FullscreenContext";
import {
  ArrowLeft,
  ArrowRight,
  Eye,
  Pin,
  Settings,
  Share2,
} from "lucide-react-native";
import { useEffect, useState } from "react";
import {
  Platform,
  Pressable,
  ScrollView,
  type LayoutChangeEvent,
} from "react-native";
import { useStore } from "store";
import { useEmojiData } from "utils/emoji";

/**
 * The "card" stream layout (branding key streamLayout=card): the stream as a
 * post in a 600px feed column with a live-chat column beside it, the way a
 * social timeline presents a video. Two columns from 1040px of content
 * width; below that the chat stacks under the post and the page scrolls.
 *
 * Everything here is chrome around the existing Player, Chat and ChatBox;
 * the video, chat state and moderation are untouched.
 */

const FEED_WIDTH = 600;
const CHAT_WIDTH = 407;
// Feed + chat + the hairline between them; the social shell sizes its
// content column to exactly this.
const TWO_COLUMN_MIN = FEED_WIDTH + CHAT_WIDTH + 1;

function formatPostDate(iso: string | undefined): string | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  const date = d.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
  const time = d.toLocaleTimeString(undefined, {
    hour: "numeric",
    minute: "2-digit",
  });
  return `${date} at ${time}`;
}

function Hairline({ vertical = false }: { vertical?: boolean }) {
  const { theme } = useTheme();
  return (
    <View
      style={
        vertical
          ? {
              width: 1,
              alignSelf: "stretch",
              backgroundColor: theme.colors.borderSubtle,
            }
          : {
              height: 1,
              width: "100%",
              backgroundColor: theme.colors.borderSubtle,
            }
      }
    />
  );
}

function IconCircleButton({
  onPress,
  label,
  children,
}: {
  onPress: () => void;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <Pressable
      accessibilityLabel={label}
      accessibilityRole="button"
      onPress={onPress}
      style={({ hovered }: any) => ({
        width: 33,
        height: 33,
        borderRadius: 999,
        alignItems: "center",
        justifyContent: "center",
        opacity: hovered ? 0.8 : 1,
      })}
    >
      {children}
    </Pressable>
  );
}

function CardHeader() {
  const { theme } = useTheme();
  const navigation: any = useNavigation();
  const goBack = () => {
    if (navigation.canGoBack()) navigation.goBack();
    else
      navigation.navigate("MainTabs", {
        screen: "HomeTab",
        params: { screen: "HomeMain" },
      });
  };
  return (
    <View
      style={{
        height: 52,
        flexDirection: "row",
        alignItems: "center",
        paddingHorizontal: 20,
        gap: 8,
      }}
    >
      <IconCircleButton onPress={goBack} label="Back">
        <ArrowLeft size={22} color={theme.colors.text2} />
      </IconCircleButton>
      <Text
        weight="semibold"
        style={{ flex: 1, fontSize: 19, lineHeight: 22 }}
        numberOfLines={1}
      >
        Live
      </Text>
      <IconCircleButton
        onPress={() =>
          navigation.navigate("MainTabs", {
            screen: "SettingsTab",
            params: { screen: "Settings" },
          })
        }
        label="Settings"
      >
        <Settings size={22} color={theme.colors.text2} />
      </IconCircleButton>
    </View>
  );
}

// A quiet offline state sized to the embed box, in place of the player's
// full-screen offline card (which lays itself out against the window).
function CardOffline({ handle }: { handle: string }) {
  const { theme } = useTheme();
  return (
    <View
      style={{
        flex: 1,
        alignItems: "center",
        justifyContent: "center",
        padding: 24,
        backgroundColor: theme.colors.surface1,
      }}
    >
      <Text
        weight="semibold"
        style={{ fontSize: 17, lineHeight: 22, textAlign: "center" }}
      >
        {handle} is offline right now
      </Text>
      <Text
        style={{
          fontSize: 15,
          lineHeight: 20,
          marginTop: 6,
          color: theme.colors.text2,
          textAlign: "center",
        }}
      >
        Check back later.
      </Text>
    </View>
  );
}

function PostCard({
  src,
  extraProps,
  onTeleport,
  compact = false,
}: {
  src: string;
  extraProps: Partial<PlayerProps>;
  onTeleport?: (targetHandle: string, targetDID: string) => void;
  /** Streamer row, title and player only: the fixed phone layout keeps the
   *  rest of the height for chat. */
  compact?: boolean;
}) {
  const { theme } = useTheme();
  const toast = useToast();
  const { title, avatarUri, views, displayName, handleStr } = useStreamMeta();
  // Liveness from the segments actually arriving (a fresh one in the last
  // ten seconds), not the player's mode; no segment at all means offline.
  const segment = useLivestreamStore((x) => x.segment);
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, []);
  const isLive =
    !!segment?.startTime && now - Date.parse(segment.startTime) < 10_000;
  const ls = useLivestreamStore((x) => x.livestream);
  const postDate = formatPostDate((ls?.record as any)?.createdAt);
  const name = displayName || handleStr;

  const share = async () => {
    try {
      if (Platform.OS === "web" && typeof navigator !== "undefined") {
        const url = window.location.href;
        if ((navigator as any).share) {
          await (navigator as any).share({ url });
        } else {
          await navigator.clipboard.writeText(url);
          toast.show("Link copied", undefined, { variant: "success" });
        }
      }
    } catch {
      // user dismissed the share sheet
    }
  };

  return (
    <View style={{ padding: 16, gap: 0 }}>
      {/* streamer row */}
      <View style={{ flexDirection: "row", gap: 12, alignItems: "center" }}>
        <Avatar src={avatarUri} name={name} size={42} live={isLive} />
        <View style={{ flex: 1, minWidth: 0 }}>
          <Text
            weight="semibold"
            numberOfLines={1}
            style={{ fontSize: 17, lineHeight: 22 }}
          >
            {name}
          </Text>
          {displayName ? (
            <Text
              numberOfLines={1}
              style={{
                fontSize: 15,
                lineHeight: 20,
                color: theme.colors.text2,
              }}
            >
              {handleStr}
            </Text>
          ) : null}
        </View>
      </View>

      {/* the post text is the stream title */}
      {title ? (
        <Text
          numberOfLines={compact ? 2 : undefined}
          style={{ fontSize: 17, lineHeight: 22, marginTop: 12 }}
        >
          {title}
        </Text>
      ) : null}

      {/* the embed is the player */}
      <View
        style={{
          marginTop: 12,
          width: "100%",
          aspectRatio: 16 / 9,
          borderRadius: 12,
          overflow: "hidden",
          backgroundColor: "#000", // token-ok: video letterbox
        }}
      >
        <KeepAwake />
        <FullscreenProvider>
          <Player
            key={src}
            src={src}
            {...extraProps}
            hideChat
            fitContainer
            offlineContent={<CardOffline handle={handleStr} />}
            onTeleport={onTeleport}
          />
        </FullscreenProvider>
      </View>

      {/* engagement row */}
      {!compact && (
        <View
          style={{
            flexDirection: "row",
            alignItems: "center",
            gap: 16,
            paddingVertical: 12,
            marginTop: 8,
          }}
        >
          <View style={{ flexDirection: "row", alignItems: "center", gap: 4 }}>
            <Eye size={18} color={theme.colors.text3} />
            <Text
              style={{
                fontSize: 13,
                lineHeight: 17,
                color: theme.colors.text3,
              }}
            >
              {typeof views === "number" ? views : 0}
              {isLive ? " watching" : ""}
            </Text>
          </View>
          {isLive ? (
            <View
              style={{ flexDirection: "row", alignItems: "center", gap: 6 }}
            >
              <View
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: 4,
                  backgroundColor: theme.colors.live,
                }}
              />
              <Text weight="semibold" style={{ fontSize: 13, lineHeight: 17 }}>
                Live
              </Text>
            </View>
          ) : (
            <Text
              style={{
                fontSize: 13,
                lineHeight: 17,
                color: theme.colors.text3,
              }}
            >
              Offline
            </Text>
          )}
          <View style={{ flex: 1 }} />
          <IconCircleButton onPress={share} label="Share">
            <Share2 size={18} color={theme.colors.text3} />
          </IconCircleButton>
        </View>
      )}

      {postDate && !compact ? (
        <Text
          style={{
            fontSize: 13,
            lineHeight: 17,
            color: theme.colors.text2,
            paddingVertical: 12,
          }}
        >
          {postDate}
        </Text>
      ) : null}
    </View>
  );
}

function PinnedCard() {
  const { theme } = useTheme();
  const pinned = useLivestreamStore((x) => x.pinnedComment);
  if (!pinned?.message) return null;
  const message: any = pinned.message;
  const author = message.author ?? {};
  const name: string = author.displayName || author.handle || "";
  const handle: string = author.handle ? `@${author.handle}` : "";
  const text: string = message.record?.text ?? "";
  return (
    <View
      style={{
        borderWidth: 1,
        borderColor: theme.colors.borderStrong,
        borderRadius: 12,
        overflow: "hidden",
      }}
    >
      <View
        style={{
          flexDirection: "row",
          alignItems: "center",
          gap: 4,
          paddingHorizontal: 8,
          paddingTop: 8,
          paddingLeft: 70,
        }}
      >
        <Pin size={16} color={theme.colors.text3} />
        <Text
          weight="medium"
          style={{ fontSize: 13, lineHeight: 17, color: theme.colors.text2 }}
        >
          Pinned
        </Text>
      </View>
      <View
        style={{ flexDirection: "row", gap: 12, padding: 12, paddingTop: 8 }}
      >
        <Avatar src={author.avatar} name={name} size={42} />
        <View style={{ flex: 1, minWidth: 0 }}>
          <Text numberOfLines={1} style={{ fontSize: 15, lineHeight: 23 }}>
            <Text weight="semibold" style={{ fontSize: 15 }}>
              {name}
            </Text>
            {handle ? (
              <Text style={{ fontSize: 15, color: theme.colors.text2 }}>
                {"  " + handle}
              </Text>
            ) : null}
          </Text>
          <Text style={{ fontSize: 15, lineHeight: 23 }}>{text}</Text>
        </View>
      </View>
    </View>
  );
}

function CardChatPanel({ fill }: { fill: boolean }) {
  const { theme } = useTheme();
  const agent = usePDSAgent();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const emojiData = useEmojiData();
  const customEmoji: any[] = [];

  return (
    <View
      style={{
        flex: fill ? 1 : undefined,
        height: fill ? undefined : 560,
        padding: 20,
        gap: 16,
      }}
    >
      <Text weight="semibold" style={{ fontSize: 15, lineHeight: 23 }}>
        Live chat
      </Text>
      <PinnedCard />
      <View style={{ flex: 1, minHeight: 0 }}>
        <Chat />
      </View>
      {agent?.did ? (
        <ChatBox
          emojiData={emojiData}
          // The timeline composer: a 44px field, no send button (Enter
          // sends), a chat glyph at the left in place of the badge picker.
          chatBoxStyle={{
            borderRadius: 10,
            backgroundColor: theme.colors.surface2,
            borderWidth: 0,
            height: 44,
            paddingLeft: 12,
            gap: 4,
          }}
          placeholder="Write your message..."
          hideSendButton
          hideToolbar
          leftSlot={<MessageIcon size={18} color={theme.colors.text3} />}
          emojiPicker={(isOpen, onClose, onSelect) => (
            <EmojiPicker
              isOpen={isOpen}
              onClose={onClose}
              onSelect={onSelect}
              customEmoji={customEmoji}
            />
          )}
        />
      ) : !agent ? (
        <View
          style={[
            zero.layout.flex.row,
            zero.layout.flex.center,
            {
              padding: 12,
              borderRadius: 10,
              backgroundColor: theme.colors.surface2,
            },
          ]}
        >
          <Loader size="large" />
        </View>
      ) : (
        <Pressable
          onPress={() => openLoginModal()}
          style={[
            zero.layout.flex.row,
            zero.layout.flex.center,
            {
              gap: 8,
              paddingVertical: 12,
              paddingHorizontal: 12,
              borderRadius: 10,
              backgroundColor: theme.colors.surface2,
            },
          ]}
        >
          <Text style={{ fontSize: 15 }}>Log in to write a message</Text>
          <ArrowRight color={theme.colors.text1} size={16} />
        </Pressable>
      )}
    </View>
  );
}

export function StreamCardLayout({
  src,
  extraProps,
  onTeleport,
}: {
  src: string;
  extraProps: Partial<PlayerProps>;
  onTeleport?: (targetHandle: string, targetDID: string) => void;
}) {
  const { theme } = useTheme();
  // Measure the content area rather than the window: the app's sidebar
  // takes a slice of the window on web.
  const [contentWidth, setContentWidth] = useState(0);
  const onLayout = (e: LayoutChangeEvent) =>
    setContentWidth(e.nativeEvent.layout.width);
  const twoColumn = contentWidth >= TWO_COLUMN_MIN;
  // In the social shell the nav rail's own border is the feed's left edge.
  const socialShell = useSocialShell();

  if (twoColumn) {
    return (
      <View
        onLayout={onLayout}
        style={{
          flex: 1,
          flexDirection: "row",
          justifyContent: socialShell ? "flex-start" : "center",
          backgroundColor: theme.colors.background,
        }}
      >
        {!socialShell && <Hairline vertical />}
        <ScrollView
          style={{ width: FEED_WIDTH, flexGrow: 0 }}
          contentContainerStyle={{ paddingBottom: 40 }}
        >
          <CardHeader />
          <Hairline />
          <PostCard src={src} extraProps={extraProps} onTeleport={onTeleport} />
          <Hairline />
        </ScrollView>
        <Hairline vertical />
        <View style={{ width: CHAT_WIDTH, alignSelf: "stretch" }}>
          <CardChatPanel fill />
        </View>
      </View>
    );
  }

  // Narrow: a fixed column — the player up top, chat filling the rest, the
  // composer pinned at the bottom — rather than a page that scrolls.
  return (
    <View
      onLayout={onLayout}
      style={{
        flex: 1,
        alignItems: "center",
        backgroundColor: theme.colors.background,
      }}
    >
      <View style={{ flex: 1, width: "100%", maxWidth: FEED_WIDTH }}>
        <CardHeader />
        <Hairline />
        <PostCard
          src={src}
          extraProps={extraProps}
          onTeleport={onTeleport}
          compact
        />
        <Hairline />
        <CardChatPanel fill />
      </View>
    </View>
  );
}
