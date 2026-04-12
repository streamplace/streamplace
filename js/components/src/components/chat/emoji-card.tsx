import { TriggerRef } from "@rn-primitives/dropdown-menu";
import { Brush } from "lucide-react-native";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Image, Linking, Platform, Pressable, View } from "react-native";
import { EmoteView } from "streamplace/src/lexicons/types/place/stream/richtext/facet";
import { formatHandle, zero } from "../..";
import { useAvatars } from "../../hooks/useAvatars";
import { borders, r, shadows } from "../../lib/theme/atoms";
import { useTheme } from "../../ui";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "../ui/dropdown";
import { Text } from "../ui/text";

const EMOTE_IMAGE_SIZE = 28;

export interface OpenEmojiCardData {
  cardKey: string;
  emote: EmoteView;
  anchorX: number;
  anchorY: number;
}

interface OpenEmojiCardContextValue {
  openCard: OpenEmojiCardData | null;
  setOpenCard: (card: OpenEmojiCardData | null) => void;
}

export const OpenEmojiCardContext = createContext<OpenEmojiCardContextValue>({
  openCard: null,
  setOpenCard: () => {},
});

// All hook-derived data needed to render the card — computed outside any Modal boundary.
interface EmojiCardData {
  emote: EmoteView;
  ownerProfile: ReturnType<typeof useAvatars>[string] | undefined;
  emoteCreatorProfile: ReturnType<typeof useAvatars>[string] | undefined;
}

function useEmojiCardData(emote: EmoteView): EmojiCardData {
  const ownerDid = emote.record.uri.split("/")[2];
  const emoteCreatorDid = emote.record.creator;

  const allDids = useMemo(() => {
    const dids = [ownerDid];
    if (emoteCreatorDid) dids.push(emoteCreatorDid);
    return dids;
  }, [ownerDid, emoteCreatorDid]);

  const profiles = useAvatars(allDids);

  return {
    emote,
    ownerProfile: profiles[ownerDid],
    emoteCreatorProfile: emoteCreatorDid
      ? profiles[emoteCreatorDid]
      : undefined,
  };
}

const EmojiCardContent = ({
  data,
  theme,
}: {
  data: EmojiCardData;
  theme: ReturnType<typeof useTheme>["theme"];
}) => {
  const { emote, ownerProfile, emoteCreatorProfile } = data;

  return (
    <View style={[zero.py[3], zero.gap.all[3]]}>
      <View
        style={[
          zero.gap.all[1],
          zero.layout.flex.row,
          zero.layout.flex.alignCenter,
        ]}
      >
        <View style={[zero.layout.flex.align.start]}>
          <Image
            source={{ uri: emote.record.imageUrl }}
            style={[zero.w[16], zero.h[16]]}
            accessibilityLabel={emote.record.alt ?? emote.record.name}
          />
        </View>
        <View>
          <Text>:{emote.record.name}:</Text>
          {emoteCreatorProfile ? (
            <Pressable
              style={[
                zero.layout.flex.row,
                zero.layout.flex.alignCenter,
                zero.gap.column[1],
              ]}
              onPress={() => {
                // just link to bsky for now to get around internal linking issues in components
                Linking.openURL(
                  `https://bsky.app/profile/${emoteCreatorProfile.did}`,
                );
              }}
            >
              <Brush size={5 * 4} color={theme.colors.primary} />
              {emoteCreatorProfile.avatar && (
                <Image
                  source={{ uri: emoteCreatorProfile.avatar }}
                  style={[zero.w[5], zero.h[5], { borderRadius: 999 }]}
                />
              )}
              <Text color="primary">{formatHandle(emoteCreatorProfile)}</Text>
            </Pressable>
          ) : null}
        </View>
      </View>
      {ownerProfile ? (
        <Pressable
          style={[
            zero.layout.flex.row,
            zero.layout.flex.alignCenter,
            zero.gap.column[2],
          ]}
          onPress={() => {
            // just link to bsky for now to get around internal linking issues in components
            Linking.openURL(`https://bsky.app/profile/${ownerProfile.did}`);
          }}
        >
          {ownerProfile.avatar ? (
            <Image
              source={{ uri: ownerProfile.avatar }}
              style={[zero.w[5], zero.h[5], { borderRadius: 10 }]}
            />
          ) : (
            <View
              style={[
                zero.w[10],
                zero.h[5],
                {
                  borderRadius: 10,
                  backgroundColor: theme.colors.mutedForeground,
                },
              ]}
            />
          )}
          <Text size="xs" color="muted">
            from @{ownerProfile.handle}
          </Text>
        </Pressable>
      ) : null}
    </View>
  );
};

// Web only overlay rendered in a React portal
const EmojiCardOverlay = ({
  card,
  onClose,
}: {
  card: OpenEmojiCardData;
  onClose: () => void;
}) => {
  const { theme } = useTheme();
  const data = useEmojiCardData(card.emote);

  const [portalContainer, setPortalContainer] = useState<Element | null>(null);
  useEffect(() => {
    if (typeof document !== "undefined") {
      setPortalContainer(document.body);
    }
  }, []);

  if (!portalContainer) return null;

  const viewportWidth = typeof window !== "undefined" ? window.innerWidth : 400;
  const viewportHeight =
    typeof window !== "undefined" ? window.innerHeight : 600;
  const cardWidth = 280;
  const left = Math.max(
    8,
    Math.min(card.anchorX, viewportWidth - cardWidth - 8),
  );
  const flipUp = viewportHeight - card.anchorY < 200;
  const verticalStyle = flipUp
    ? { bottom: viewportHeight - card.anchorY + 4 }
    : { top: card.anchorY + 4 };

  const { createPortal } = require("react-dom");

  return createPortal(
    <>
      {/* Invisible backdrop — clicking outside closes the card */}
      <Pressable
        onPress={onClose}
        style={{
          position: "fixed" as any,
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          zIndex: 9998,
        }}
      />
      <Pressable
        style={[
          r.md,
          borders.width.thin,
          shadows.lg,
          {
            position: "fixed" as any,
            left,
            ...verticalStyle,
            width: cardWidth,
            zIndex: 9999,
            backgroundColor: theme.colors.popover,
            borderColor: theme.colors.border,
            overflow: "hidden",
            padding: 8,
          },
        ]}
      >
        <EmojiCardContent data={data} theme={theme} />
      </Pressable>
    </>,
    portalContainer,
  );
};

export const EmojiCardProvider = ({
  children,
}: {
  children: React.ReactNode;
}) => {
  const [openCard, setOpenCard] = useState<OpenEmojiCardData | null>(null);
  const value = useMemo(() => ({ openCard, setOpenCard }), [openCard]);
  return (
    <OpenEmojiCardContext.Provider value={value}>
      {children}
      {openCard && Platform.OS === "web" && (
        <EmojiCardOverlay card={openCard} onClose={() => setOpenCard(null)} />
      )}
    </OpenEmojiCardContext.Provider>
  );
};

export const EmojiCard = ({
  emote,
  cardKey,
}: {
  emote: EmoteView;
  cardKey: string;
}) => {
  const { theme } = useTheme();
  const data = useEmojiCardData(emote);
  const { setOpenCard } = useContext(OpenEmojiCardContext);
  const [hovered, setHovered] = useState(false);
  // web: ref for measuring anchor position
  const triggerRef = useRef<View>(null);
  // native: ref for the dropdown trigger
  const dropdownRef = useRef<TriggerRef>(null);

  const openWebCard = useCallback(() => {
    if (triggerRef.current) {
      triggerRef.current.measureInWindow((x, y, width, height) => {
        setOpenCard({ cardKey, emote, anchorX: x, anchorY: y + height });
      });
    }
  }, [cardKey, emote, setOpenCard]);

  // Native: use DropdownMenu for built-in positioning and interactions.
  // * important ! all data must be computed outside the dropdown and passed in!
  if (Platform.OS !== "web") {
    return (
      <DropdownMenu>
        <DropdownMenuTrigger ref={dropdownRef} asChild>
          <Pressable onPress={() => {}}>
            <Image
              source={{ uri: emote.record.imageUrl }}
              accessibilityLabel={emote.name}
              style={{ height: EMOTE_IMAGE_SIZE, width: EMOTE_IMAGE_SIZE }}
            />
          </Pressable>
        </DropdownMenuTrigger>
        <DropdownMenuContent style={{ minWidth: 200, maxWidth: 280 }}>
          <EmojiCardContent data={data} theme={theme} />
        </DropdownMenuContent>
      </DropdownMenu>
    );
  }

  // Web: Pressable that pushes card data + anchor coords into context for the portal overlay.
  return (
    <Pressable
      ref={triggerRef}
      onPress={openWebCard}
      {...{
        onHoverIn: () => setHovered(true),
        onHoverOut: () => setHovered(false),
      }}
      // awful awful awful alignment
      style={
        {
          display: "inline",
          verticalAlign: "top",
          alignItems: "center",
          padding: 2,
          margin: -2,
          marginTop: -EMOTE_IMAGE_SIZE,
          top: EMOTE_IMAGE_SIZE * 0.3,
          ...(hovered
            ? { backgroundColor: "rgba(255,255,255,0.15)", borderRadius: 6 }
            : {}),
        } as any
      }
    >
      <Image
        source={{ uri: emote.record.imageUrl }}
        accessibilityLabel={emote.name}
        style={
          {
            display: "inline-flex",
            verticalAlign: "middle",
            alignItems: "center",
            height: EMOTE_IMAGE_SIZE,
            width: EMOTE_IMAGE_SIZE,
          } as any
        }
      />
    </Pressable>
  );
};
