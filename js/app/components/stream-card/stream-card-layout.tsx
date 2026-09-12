import { useNavigation } from "@react-navigation/native";
import {
  Avatar,
  Chat,
  ChatBox,
  KeepAwake,
  Loader,
  Text,
  useAccessStatus,
  useBrandingAsset,
  useChatLockedOut,
  useLivestreamStore,
  useSocialShell,
  useTheme,
  useToast,
  View,
  zero,
} from "@streamplace/components";
import { RichTextMessage } from "@streamplace/components/src/components/chat/chat-message";
import { VerifiedBadge } from "@streamplace/components/src/components/chat/verified-badge";
import { useAvatars } from "@streamplace/components/src/hooks/useAvatars";
import { useUnpinChatMessage } from "@streamplace/components/src/livestream-store/hooks";
import { useCanModerate } from "@streamplace/components/src/streamplace-store/moderation";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { EmojiPicker } from "components/emoji-picker/emoji-picker";
import { useStreamMeta } from "components/mobile/bottom-metadata";
import { Player } from "components/mobile/player";
import { PlayerProps } from "components/player/props";
import { PhoneMenuButton } from "components/sidebar/sidebar-overlay";
import { MessageIcon } from "components/sidebar/social-icons";
import { FullscreenProvider } from "contexts/FullscreenContext";
import { usePhoneMenu } from "hooks/useSidebarControl";
import { ArrowLeft, Eye, Pin, Share2, X } from "lucide-react-native";
import { useEffect, useMemo, useState } from "react";
import {
  Linking,
  Platform,
  Pressable,
  ScrollView,
  useWindowDimensions,
  type LayoutChangeEvent,
} from "react-native";
import { useStore } from "store";
import { useEmojiData } from "utils/emoji";
import { chatWidthFor, FEED_WIDTH } from "./widths";

/**
 * The "card" stream layout (branding key streamLayout=card): the stream as a
 * post in a 600px feed column with a live-chat column beside it, the way a
 * social timeline presents a video. Two columns from 1040px of content
 * width; below that the chat stacks under the post and the page scrolls.
 *
 * Everything here is chrome around the existing Player, Chat and ChatBox;
 * the video, chat state and moderation are untouched.
 */

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
  const phoneMenu = usePhoneMenu();
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
      {phoneMenu && <PhoneMenuButton style={{ marginRight: -8 }} />}
    </View>
  );
}

// The feed column's tab bar from the design: Live, Video on demand and Go
// live. It belongs to a stream index page, not to a stream's own page, so
// nothing renders it until that index exists. 48px, 15px labels, a 3px
// brand-colored indicator.
export function CardTabs() {
  const { theme } = useTheme();
  const navigation: any = useNavigation();
  const tabs: { label: string; active?: boolean; onPress: () => void }[] = [
    { label: "Live", active: true, onPress: () => {} },
    {
      label: "Video on demand",
      onPress: () => navigation.navigate("MainTabs", { screen: "VideosTab" }),
    },
    {
      label: "Go live",
      onPress: () =>
        navigation.navigate("MainTabs", {
          screen: "HomeTab",
          params: { screen: "LiveDashboard" },
        }),
    },
  ];
  return (
    <View style={{ flexDirection: "row", height: 48 }}>
      {tabs.map((tab) => (
        <Pressable
          key={tab.label}
          accessibilityRole="tab"
          accessibilityState={{ selected: !!tab.active }}
          onPress={tab.onPress}
          style={({ hovered }: any) => ({
            flex: 1,
            alignItems: "center",
            justifyContent: "flex-end",
            paddingTop: 14,
            opacity: hovered && !tab.active ? 0.85 : 1,
          })}
        >
          <Text
            weight="semibold"
            style={{
              fontSize: 15,
              lineHeight: 20,
              color: tab.active ? theme.colors.text1 : theme.colors.text2,
            }}
          >
            {tab.label}
          </Text>
          <View
            style={{
              marginTop: 10,
              height: 3,
              alignSelf: "stretch",
              marginHorizontal: 16,
              borderRadius: 2,
              backgroundColor: tab.active
                ? theme.colors.primary
                : "transparent",
            }}
          />
        </Pressable>
      ))}
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

// The embed: the player in a 16:9 box, rounded inside the feed card and
// edge to edge on phones.
function PlayerEmbed({
  src,
  extraProps,
  onTeleport,
  rounded = true,
}: {
  src: string;
  extraProps: Partial<PlayerProps>;
  onTeleport?: (targetHandle: string, targetDID: string) => void;
  rounded?: boolean;
}) {
  const { handleStr } = useStreamMeta();
  return (
    <View
      style={{
        width: "100%",
        aspectRatio: 16 / 9,
        borderRadius: rounded ? 12 : 0,
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
  );
}

function PostCard({
  src,
  extraProps,
  onTeleport,
  compact = false,
  phone = false,
}: {
  src: string;
  extraProps: Partial<PlayerProps>;
  onTeleport?: (targetHandle: string, targetDID: string) => void;
  /** Streamer row, title and player only: the fixed phone layout keeps the
   *  rest of the height for chat. */
  compact?: boolean;
  /** The phone design: the embed sits above the card edge to edge, so the
   *  card is the streamer row, text, engagement and date alone. */
  phone?: boolean;
}) {
  const { theme } = useTheme();
  const toast = useToast();
  const { title, avatarUri, views, displayName, handleStr } = useStreamMeta();
  // The node reports the streamer's verification on the profile it sends
  // over the stream's websocket, the same way it does for chat authors.
  const streamer = useLivestreamStore((x) => x.profile);
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
          <View style={{ flexDirection: "row", alignItems: "center" }}>
            <Text
              weight="semibold"
              numberOfLines={1}
              style={{ fontSize: 17, lineHeight: 22, flexShrink: 1 }}
            >
              {name}
            </Text>
            <VerifiedBadge author={(streamer as any) ?? {}} size={16} />
          </View>
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

      {/* the embed is the player; the post text sits under it so a long
          title doesn't push the video down */}
      {!phone && (
        <View style={{ marginTop: 12 }}>
          <PlayerEmbed
            src={src}
            extraProps={extraProps}
            onTeleport={onTeleport}
          />
        </View>
      )}
      {title ? (
        <Text
          numberOfLines={compact ? 2 : undefined}
          style={{ fontSize: 17, lineHeight: 22, marginTop: 12 }}
        >
          {title}
        </Text>
      ) : null}
      {phone && <Hairline />}

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

// The pinned message: a filled card one surface step above its container
// (the design's #1d2433 on the page, #272f43 inside the phone chat sheet).
function PinnedCard({ raised = false }: { raised?: boolean }) {
  const { theme } = useTheme();
  const toast = useToast();
  const pinned = useLivestreamStore((x) => x.pinnedComment);
  const streamerDID = useLivestreamStore((x) => x.profile?.did);
  const message: any = pinned?.message;
  const author = message?.author ?? {};
  // The pinned record carries the author's handle but not avatar or
  // display name; the profile cache fills them in like chat rows do.
  const dids = useMemo(() => (author.did ? [author.did] : []), [author.did]);
  const profile = useAvatars(dids)[author.did];
  // The streamer and moderators with the pin permission can take it down.
  const canUnpin = !!useCanModerate(streamerDID ?? "")?.canPin;
  const unpin = useUnpinChatMessage();
  const onUnpin = async () => {
    if (!pinned?.uri || !streamerDID) return;
    try {
      await unpin(pinned.uri, streamerDID);
    } catch (e) {
      toast.show(
        "Couldn't unpin",
        e instanceof Error ? e.message : "Failed to unpin",
        { variant: "error" },
      );
    }
  };
  if (!message) return null;
  const avatar: string | undefined = profile?.avatar || author.avatar;
  const name: string =
    author.displayName || profile?.displayName || author.handle || "";
  const handle: string = author.handle ? `@${author.handle}` : "";
  const text: string = message.record?.text ?? "";
  const facets = message.record?.facets ?? [];
  return (
    <View
      style={{
        backgroundColor: raised ? theme.colors.surface2 : theme.colors.surface1,
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
          style={{
            flex: 1,
            fontSize: 13,
            lineHeight: 17,
            color: theme.colors.text2,
          }}
        >
          Pinned
        </Text>
        {canUnpin && (
          <Pressable
            onPress={onUnpin}
            accessibilityRole="button"
            accessibilityLabel="Unpin message"
            hitSlop={8}
            style={({ hovered }: any) => ({
              width: 24,
              height: 24,
              borderRadius: 999,
              alignItems: "center",
              justifyContent: "center",
              opacity: hovered ? 1 : 0.7,
            })}
          >
            <X size={14} color={theme.colors.text2} />
          </Pressable>
        )}
      </View>
      <View
        style={{ flexDirection: "row", gap: 12, padding: 12, paddingTop: 8 }}
      >
        <Avatar src={avatar} name={name} size={42} />
        <View style={{ flex: 1, minWidth: 0 }}>
          <Text numberOfLines={1} style={{ fontSize: 15, lineHeight: 23 }}>
            <Text weight="semibold" style={{ fontSize: 15 }}>
              {name}
            </Text>
            <VerifiedBadge author={author} profile={profile} size={14} />
            {handle && name !== author.handle ? (
              <Text style={{ fontSize: 15, color: theme.colors.text2 }}>
                {"  " + handle}
              </Text>
            ) : null}
          </Text>
          <Text style={{ fontSize: 15, lineHeight: 23 }}>
            <RichTextMessage text={text} facets={facets} />
          </Text>
        </View>
      </View>
    </View>
  );
}

// The composer's three states from the design: signed out (the field opens
// login, "Verified users can write messages. Sign in"), signed in but not
// verified while chat is verified-only ("... Verify now", to the node's
// verifyUrl), and able to write (the field alone).
function ComposerNotice({
  text,
  linkLabel,
  onPress,
}: {
  text: string;
  linkLabel?: string;
  onPress?: () => void;
}) {
  const { theme } = useTheme();
  return (
    <Text
      style={{
        fontSize: 13,
        lineHeight: 17,
        color: theme.colors.text2,
        textAlign: "center",
        marginTop: -6,
      }}
    >
      {text}
      {linkLabel && onPress ? (
        <>
          {" "}
          <Text
            onPress={onPress}
            style={{
              fontSize: 13,
              lineHeight: 17,
              color: theme.colors.primary,
            }}
          >
            {linkLabel}
          </Text>
        </>
      ) : null}
    </Text>
  );
}

// A look-alike of the composer field for the states that can't take input.
function ComposerPlaceholder({ onPress }: { onPress?: () => void }) {
  const { theme } = useTheme();
  return (
    <Pressable
      onPress={onPress}
      disabled={!onPress}
      accessibilityRole={onPress ? "button" : undefined}
      style={{
        flexDirection: "row",
        alignItems: "center",
        gap: 4,
        height: 44,
        paddingLeft: 12,
        paddingRight: 12,
        borderRadius: 10,
        backgroundColor: theme.colors.surface2,
      }}
    >
      <MessageIcon size={18} color={theme.colors.text3} />
      <Text style={{ fontSize: 15, color: theme.colors.text3, marginLeft: 4 }}>
        Write your message...
      </Text>
    </Pressable>
  );
}

function CardChatPanel({
  fill,
  raised = false,
  bare = false,
}: {
  fill: boolean;
  /** Inside the phone chat sheet: the pinned card one surface step up. */
  raised?: boolean;
  /** No "Live chat" heading (the sheet draws its own header). */
  bare?: boolean;
}) {
  const { theme } = useTheme();
  const agent = usePDSAgent();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const emojiData = useEmojiData();
  const customEmoji: any[] = [];
  const verifiedOnly = !!useAccessStatus()?.chatVerifiedOnly;
  const lockedOut = useChatLockedOut();
  const verifyUrl = useBrandingAsset("verifyUrl")?.data?.trim();
  const verifiedOnlyMessage =
    useBrandingAsset("chatVerifiedOnlyMessage")?.data?.trim() ||
    "Verified users can write messages.";
  const verifyLinkLabel =
    useBrandingAsset("verifyLinkLabel")?.data?.trim() || "Verify now";

  return (
    <View
      style={{
        flex: fill ? 1 : undefined,
        height: fill ? undefined : 560,
        padding: bare ? 16 : 20,
        paddingTop: bare ? 0 : 20,
        gap: 16,
      }}
    >
      {!bare && (
        <Text weight="semibold" style={{ fontSize: 15, lineHeight: 23 }}>
          Live chat
        </Text>
      )}
      <PinnedCard raised={raised} />
      <View style={{ flex: 1, minHeight: 0 }}>
        <Chat />
      </View>
      {agent?.did && lockedOut ? (
        <>
          <ComposerPlaceholder />
          <ComposerNotice
            text={verifiedOnlyMessage}
            linkLabel={verifyUrl ? verifyLinkLabel : undefined}
            onPress={verifyUrl ? () => Linking.openURL(verifyUrl) : undefined}
          />
        </>
      ) : agent?.did ? (
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
        <>
          <ComposerPlaceholder onPress={() => openLoginModal()} />
          <ComposerNotice
            text={
              verifiedOnly ? verifiedOnlyMessage : "Sign in to write messages."
            }
            linkLabel="Sign in"
            onPress={() => openLoginModal()}
          />
        </>
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
  // Feed + chat + the hairline between them; the social shell sizes its
  // content column to exactly this, and the chat widens on big windows.
  const { width: windowWidth } = useWindowDimensions();
  const chatWidth = chatWidthFor(windowWidth);
  const twoColumn = contentWidth >= FEED_WIDTH + chatWidth + 1;
  const [chatOpen, setChatOpen] = useState(false);
  // In the social shell the nav rail's own border is the feed's left edge.
  const socialShell = useSocialShell();
  // Tell the shell this page wants the feed + chat column while mounted
  // (route names alone misfire, e.g. on the return from login).
  const setWideColumn = useStore((s) => s.setWideColumn);
  useEffect(() => {
    setWideColumn(true);
    return () => setWideColumn(false);
  }, [setWideColumn]);

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
        <View style={{ width: chatWidth, alignSelf: "stretch" }}>
          <CardChatPanel fill />
        </View>
      </View>
    );
  }

  // Narrow (the phone design): the player pinned edge to edge under the
  // header, the post details scrolling beneath it with the chat minimized
  // to its last two messages; opening the chat slides a sheet up over the
  // details, the way a mobile browser shows a video's comments.
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
        <PlayerEmbed
          src={src}
          extraProps={extraProps}
          onTeleport={onTeleport}
          rounded={false}
        />
        <View style={{ flex: 1, minHeight: 0 }}>
          <ScrollView
            style={{ flex: 1 }}
            contentContainerStyle={{ paddingBottom: 28 }}
          >
            <PostCard
              src={src}
              extraProps={extraProps}
              onTeleport={onTeleport}
              phone
            />
            <Hairline />
            <MiniChat onOpen={() => setChatOpen(true)} />
          </ScrollView>
          {chatOpen && <ChatSheet onClose={() => setChatOpen(false)} />}
        </View>
      </View>
    </View>
  );
}

// The minimized chat on phones: a filled panel with the count and the last
// two messages (avatar and text), or the composer when nothing has been said
// yet. Tapping it opens the chat sheet.
function MiniChat({ onOpen }: { onOpen: () => void }) {
  const { theme } = useTheme();
  const chat = useLivestreamStore((x) => x.chat) ?? [];
  const recent = useMemo(() => chat.slice(-2), [chat]);
  const dids = useMemo(
    () => recent.map((m: any) => m.author?.did).filter(Boolean),
    [recent],
  );
  const profiles = useAvatars(dids);
  return (
    <Pressable
      onPress={onOpen}
      accessibilityRole="button"
      accessibilityLabel="Open live chat"
      style={{
        marginHorizontal: 8,
        marginTop: 8,
        padding: 12,
        paddingTop: 8,
        borderRadius: 8,
        gap: 8,
        backgroundColor: theme.colors.surface2,
      }}
    >
      <Text style={{ fontSize: 15, lineHeight: 23 }}>
        <Text weight="semibold" style={{ fontSize: 15 }}>
          Live chat
        </Text>
        {chat.length > 0 ? (
          <Text style={{ fontSize: 15, color: theme.colors.text2 }}>
            {"  " + chat.length}
          </Text>
        ) : null}
      </Text>
      {recent.length === 0 ? (
        <View
          style={{
            flexDirection: "row",
            alignItems: "center",
            gap: 4,
            height: 44,
            paddingHorizontal: 12,
            borderRadius: 10,
            backgroundColor: theme.colors.background,
          }}
        >
          <MessageIcon size={18} color={theme.colors.text3} />
          <Text
            style={{ fontSize: 15, color: theme.colors.text3, marginLeft: 4 }}
          >
            Write your message...
          </Text>
        </View>
      ) : (
        recent.map((m: any) => {
          const author = m.author ?? {};
          const profile = profiles[author.did];
          return (
            <View
              key={m.uri ?? m.cid}
              style={{ flexDirection: "row", gap: 8, alignItems: "flex-start" }}
            >
              <View style={{ paddingTop: 2 }}>
                <Avatar
                  src={profile?.avatar || author.avatar}
                  name={author.displayName || author.handle}
                  size={24}
                />
              </View>
              <Text
                numberOfLines={2}
                style={{ flex: 1, fontSize: 15, lineHeight: 23 }}
              >
                <RichTextMessage
                  text={m.record?.text ?? ""}
                  facets={m.record?.facets ?? []}
                />
              </Text>
            </View>
          );
        })
      )}
    </Pressable>
  );
}

// The maximized chat on phones: a sheet over the post details (the video
// stays put above), with a grabber, the pinned message, the chat and the
// composer.
function ChatSheet({ onClose }: { onClose: () => void }) {
  const { theme } = useTheme();
  return (
    <View
      style={{
        position: "absolute",
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        borderTopLeftRadius: 16,
        borderTopRightRadius: 16,
        backgroundColor: theme.colors.surface1,
        paddingTop: 8,
        overflow: "hidden",
      }}
    >
      <View style={{ alignItems: "center" }}>
        <View
          style={{
            width: 36,
            height: 5,
            borderRadius: 100,
            backgroundColor: theme.colors.text1,
          }}
        />
      </View>
      <View
        style={{
          flexDirection: "row",
          alignItems: "center",
          justifyContent: "space-between",
          paddingHorizontal: 16,
          height: 41,
        }}
      >
        <Text weight="semibold" style={{ fontSize: 15, lineHeight: 23 }}>
          Live chat
        </Text>
        <Pressable
          onPress={onClose}
          accessibilityRole="button"
          accessibilityLabel="Close live chat"
          style={{
            width: 33,
            height: 33,
            borderRadius: 999,
            alignItems: "center",
            justifyContent: "center",
            backgroundColor: theme.colors.surface2,
          }}
        >
          <X size={16} color={theme.colors.text1} />
        </Pressable>
      </View>
      <CardChatPanel fill raised bare />
    </View>
  );
}
