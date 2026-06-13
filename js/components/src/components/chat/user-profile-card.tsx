import { ProfileViewBasic } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { TriggerRef } from "@rn-primitives/dropdown-menu";
import { Image } from "expo-image";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Linking, Platform, Pressable, View } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { zero } from "../..";
import { useAvatars } from "../../hooks/useAvatars";
import IconBsky from "../../icons/icon-bsky";
import {
  borders,
  gap,
  h,
  pb,
  pl,
  pt,
  px,
  r,
  shadows,
  w,
} from "../../lib/theme/atoms";
import { useLivestreamStore } from "../../livestream-store";
import { useUrl } from "../../streamplace-store";
import { useTheme } from "../../ui";
import { formatHandleWithAt } from "../../utils/format-handle";
import { Button, MenuGroup } from "../ui";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "../ui/dropdown";
import { Text } from "../ui/text";
import { Badge } from "./badge";

interface BadgeMeta {
  label: string;
  description?: string;
  issuedBy?: string;
}

const BADGE_META: Record<string, BadgeMeta> = {
  "place.stream.badge.defs#mod": {
    label: "Moderator",
    issuedBy: "{issuer} for {streamer}",
  },
  "place.stream.badge.defs#bot": {
    label: "Bot",
    description: "This account has been marked as automated by its owner.",
  },
  "place.stream.badge.defs#streamer": {
    label: "Streamer",
  },
  "place.stream.badge.defs#vip": {
    label: "VIP",
    description: "This user is clearly a very important person.",
  },
};

interface OpenCardData {
  uri: string;
  author: ProfileViewBasic;
  badges: ChatMessageViewHydrated["badges"];
  anchorX: number;
  anchorY: number;
}

interface OpenCardContextValue {
  openCard: OpenCardData | null;
  setOpenCard: (card: OpenCardData | null) => void;
}

const OpenCardContext = createContext<OpenCardContextValue>({
  openCard: null,
  setOpenCard: () => {},
});

// All hook-derived data needed to render the card — computed outside any Modal boundary.
interface ProfileCardData {
  author: ProfileViewBasic;
  profile: ReturnType<typeof useAvatars>[string] | undefined;
  profiles: ReturnType<typeof useAvatars>;
  serviceDid: string | null;
  allBadges: NonNullable<ChatMessageViewHydrated["badges"]>;
  streamer: ProfileViewBasic | undefined;
}

export function useProfileCardData(
  author: ProfileViewBasic,
  badges: ChatMessageViewHydrated["badges"],
): ProfileCardData {
  const nodeUrl = useUrl();
  const serviceDid = nodeUrl
    ? `did:web:${nodeUrl.replace(/^https?:\/\//, "")}`
    : null;
  const streamer = useLivestreamStore((x) => x.livestream?.author);

  const issuerDids = useMemo(
    () =>
      badges?.map((b) => b.issuer).filter((did) => did && did !== serviceDid) ??
      [],
    [badges, serviceDid],
  );
  const allDids = useMemo(
    () => (author.did ? [author.did, ...issuerDids] : issuerDids),
    [author.did, issuerDids],
  );
  const profiles = useAvatars(allDids);

  const allBadges = (badges ?? []) as NonNullable<
    ChatMessageViewHydrated["badges"]
  >;

  return {
    author,
    profile: profiles[author.did],
    profiles,
    serviceDid,
    allBadges,
    streamer,
  };
}

const BadgeRow = ({
  streamer,
  badge,
  serviceDid,
  issuerProfiles,
}: {
  badge: NonNullable<ChatMessageViewHydrated["badges"]>[number];
  serviceDid: string;
  streamer?: ProfileViewBasic;
  issuerProfiles: ReturnType<typeof useAvatars>;
}) => {
  const isServiceIssued = badge.issuer === serviceDid;
  const meta = BADGE_META[badge.badgeType];
  const label = meta?.label ?? badge.name ?? badge.badgeType.split("#")[1];
  const description = meta?.description ?? badge.description;

  let issuerLabel = isServiceIssued
    ? "Streamplace"
    : issuerProfiles[badge.issuer]?.handle
      ? `@${issuerProfiles[badge.issuer].handle}`
      : badge.issuer;
  const issuedByTemplate = meta?.issuedBy ?? "Issued by {issuer}";
  issuerLabel = issuedByTemplate
    .replace("{issuer}", issuerLabel)
    .replace(
      "{streamer}",
      streamer?.handle ? formatHandleWithAt(streamer) : "the streamer",
    );

  return (
    <View
      style={[
        gap.all[3],
        pl[2],
        { flexDirection: "row", alignItems: "center" },
      ]}
    >
      <Badge badgeType={badge.badgeType} size={32} imageUrl={badge.imageUrl} />
      <View style={[{ flex: 1 }]}>
        <Text size="xs">{label}</Text>
        <Text size="xs" color="muted">
          {issuerLabel}
        </Text>
        {description && (
          <Text size="xs" color="muted">
            {description}
          </Text>
        )}
      </View>
    </View>
  );
};

export const ProfileCardContent = ({
  data,
  theme,
}: {
  data: ProfileCardData;
  theme: ReturnType<typeof useTheme>["theme"];
}) => {
  const { author, profile, profiles, serviceDid, allBadges, streamer } = data;

  return (
    <View style={[zero.pb[1]]}>
      {profile?.banner ? (
        <Image
          source={{ uri: profile.banner }}
          style={[h[20], { width: "100%" }, Platform.OS != "web" && zero.r.md]}
        />
      ) : (
        <View
          style={[
            h[20],
            { width: "100%", backgroundColor: theme.colors.muted },
          ]}
        />
      )}
      <View
        style={[
          px[3],
          {
            flexDirection: "row",
            alignItems: "flex-end",
            justifyContent: "space-between",
            marginTop: -24,
          },
        ]}
      >
        {profile?.avatar ? (
          <Image
            source={{ uri: profile.avatar }}
            style={[
              w[12],
              h[12],
              r.full,
              { borderWidth: 2, borderColor: theme.colors.card },
            ]}
          />
        ) : (
          <View
            style={[
              w[12],
              h[12],
              r.full,
              {
                borderWidth: 2,
                borderColor: theme.colors.card,
                backgroundColor: theme.colors.mutedForeground,
              },
            ]}
          />
        )}
      </View>
      <View style={[px[3]]}>
        <View
          style={[
            zero.layout.flex.row,
            zero.layout.flex.alignCenter,
            zero.layout.flex.justify.between,
            gap.all[2],
          ]}
        >
          <Text>@{author.handle}</Text>
          {Platform.OS === "web" && (
            <View style={{ position: "absolute", right: 2, bottom: 7 }}>
              <Button
                size="pill"
                variant="secondary"
                style={{ aspectRatio: 1 }}
                onPress={() => {
                  Linking.openURL(`https://bsky.app/profile/${author.handle}`);
                }}
              >
                <IconBsky size={18} />
              </Button>
            </View>
          )}
        </View>
        {allBadges.length > 0 ? (
          <View style={[zero.py[2]]}>
            <MenuGroup>
              {allBadges.map((badge, i) => (
                <BadgeRow
                  key={i}
                  badge={badge}
                  serviceDid={serviceDid ?? ""}
                  streamer={streamer}
                  issuerProfiles={profiles}
                />
              ))}
            </MenuGroup>
          </View>
        ) : null}
      </View>
      {Platform.OS !== "web" && (
        <View style={[px[3], pt[2]]}>
          <Button
            variant="secondary"
            size="sm"
            onPress={() => {
              Linking.openURL(`https://bsky.app/profile/${author.handle}`);
            }}
          >
            <View
              style={[
                zero.gap.all[2],
                zero.layout.flex.row,
                zero.layout.flex.alignCenter,
              ]}
            >
              <IconBsky size={20} />
              <Text>View Profile</Text>
            </View>
          </Button>
        </View>
      )}
    </View>
  );
};

// Web only overlay rendered in a React portal
const ProfileCardOverlay = ({
  card,
  onClose,
}: {
  card: OpenCardData;
  onClose: () => void;
}) => {
  const { theme } = useTheme();
  const data = useProfileCardData(card.author, card.badges);

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
  const cardWidth = 300;
  const left = Math.max(
    8,
    Math.min(card.anchorX, viewportWidth - cardWidth - 8),
  );
  const flipUp = viewportHeight - card.anchorY < 280;
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
          },
        ]}
      >
        <ProfileCardContent data={data} theme={theme} />
      </Pressable>
    </>,
    portalContainer,
  );
};

export const ProfileCardProvider = ({
  children,
}: {
  children: React.ReactNode;
}) => {
  const [openCard, setOpenCard] = useState<OpenCardData | null>(null);
  const value = useMemo(() => ({ openCard, setOpenCard }), [openCard]);
  return (
    <OpenCardContext.Provider value={value}>
      {children}
      {openCard && Platform.OS === "web" && (
        <ProfileCardOverlay card={openCard} onClose={() => setOpenCard(null)} />
      )}
    </OpenCardContext.Provider>
  );
};

export const UserProfileCard = ({
  uri,
  author,
  badges,
  children,
}: {
  uri: string;
  author: ProfileViewBasic;
  badges: ChatMessageViewHydrated["badges"];
  children: React.ReactNode;
}) => {
  const { theme } = useTheme();
  const data = useProfileCardData(author, badges);
  const { setOpenCard } = useContext(OpenCardContext);
  const [hovered, setHovered] = useState(false);
  // web: ref for measuring anchor position
  const triggerRef = useRef<View>(null);
  // native: ref for the dropdown trigger
  const dropdownRef = useRef<TriggerRef>(null);

  const openWebCard = useCallback(() => {
    if (triggerRef.current) {
      triggerRef.current.measureInWindow((x, y, width, height) => {
        setOpenCard({ uri, author, badges, anchorX: x, anchorY: y + height });
      });
    }
  }, [uri, author, badges, setOpenCard]);

  // Native: use DropdownMenu for built-in positioning and interactions.
  // * important ! all data must be computed outside the dropdown and passed in!
  if (Platform.OS !== "web") {
    return (
      <DropdownMenu>
        <DropdownMenuTrigger ref={dropdownRef} asChild>
          <Pressable
            style={{ flexDirection: "row", alignItems: "center" }}
            onPress={() => {}}
          >
            {children}
          </Pressable>
        </DropdownMenuTrigger>
        <DropdownMenuContent style={{ minWidth: 280, maxWidth: 320 }}>
          <ProfileCardContent data={data} theme={theme} />
        </DropdownMenuContent>
      </DropdownMenu>
    );
  }

  // Web: Pressable that pushes card data + anchor coords into context for the portal overlay.
  return (
    <Pressable
      ref={triggerRef}
      onPress={openWebCard}
      onHoverIn={() => setHovered(true)}
      onHoverOut={() => setHovered(false)}
      style={[
        gap.all[1],
        px[1],
        pb[1],
        hovered ? r.sm : undefined,
        {
          flexDirection: "row",
          alignItems: "center",
          marginLeft: -3,
          marginRight: -2,
          marginBottom: -4,
          ...(hovered ? { backgroundColor: "rgba(255,255,255,0.15)" } : {}),
        },
      ]}
    >
      {children}
    </Pressable>
  );
};
