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
import { Platform, Pressable, View } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { useAvatars } from "../../hooks/useAvatars";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "../ui/dropdown";
import { Text } from "../ui/text";
import { Badge } from "./badge";

interface BadgeMeta {
  label: string;
  description: string;
  issuedBy?: string;
}

const BADGE_META: Record<string, BadgeMeta> = {
  "place.stream.badge.defs#mod": {
    label: "Moderator",
    description: "This user is a moderator.",
    issuedBy: "{issuer} for {streamer}",
  },
  "place.stream.badge.defs#streamer": {
    label: "Streamer",
    description: "This user is the streamer.",
  },
  "place.stream.badge.defs#vip": {
    label: "VIP",
    description: "This user is a very important person.",
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
  closeTimeoutRef: React.MutableRefObject<ReturnType<typeof setTimeout> | null>;
}

const OpenCardContext = createContext<OpenCardContextValue>({
  openCard: null,
  setOpenCard: () => {},
  closeTimeoutRef: { current: null },
});

// All hook-derived data needed to render the card — computed outside any Modal boundary.
interface ProfileCardData {
  author: ProfileViewBasic;
  profile: ReturnType<typeof useAvatars>[string] | undefined;
  profiles: ReturnType<typeof useAvatars>;
  serviceDid: string | null;
  serviceBadges: NonNullable<ChatMessageViewHydrated["badges"]>;
  streamer: ProfileViewBasic | undefined;
}

function useProfileCardData(
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

  const serviceBadges = useMemo(
    () => badges?.filter((b) => serviceDid && b.issuer === serviceDid) ?? [],
    [badges, serviceDid],
  ) as NonNullable<ChatMessageViewHydrated["badges"]>;

  return {
    author,
    profile: profiles[author.did],
    profiles,
    serviceDid,
    serviceBadges,
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

  if (!meta) return null;

  let issuerLabel = isServiceIssued
    ? "Streamplace"
    : issuerProfiles[badge.issuer]?.handle
      ? `@${issuerProfiles[badge.issuer].handle}`
      : badge.issuer;
  if (meta.issuedBy) {
    issuerLabel = meta.issuedBy
      .replace("{issuer}", issuerLabel)
      .replace(
        "{streamer}",
        streamer?.handle ? formatHandleWithAt(streamer) : "the streamer",
      );
  }

  return (
    <View
      style={[
        gap.all[3],
        pl[2],
        { flexDirection: "row", alignItems: "center" },
      ]}
    >
      <Badge badgeType={badge.badgeType} size={32} />
      <View style={[gap.all[1], { flex: 1 }]}>
        <Text size="xs">{meta.label}</Text>
        <Text size="xs" color="muted">
          Issued by {issuerLabel}
        </Text>
        <Text size="xs" color="muted">
          {meta.description}
        </Text>
      </View>
    </View>
  );
};

// Pure UI — no hooks. All data is passed in so this can safely render inside a
// React Native Modal (which creates a new React root and loses context).
const ProfileCardContent = ({
  data,
  theme,
}: {
  data: ProfileCardData;
  theme: ReturnType<typeof useTheme>["theme"];
}) => {
  const { author, profile, profiles, serviceDid, serviceBadges, streamer } =
    data;

  return (
    <>
      {profile?.banner ? (
        <Image
          source={{ uri: profile.banner }}
          style={[h[20], { width: "100%" }]}
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
          { flexDirection: "row", alignItems: "flex-end", marginTop: -24 },
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
      <View style={[px[3], pb[3]]}>
        <Text>@{author.handle}</Text>
        {profile?.description ? (
          <Text size="sm" color="muted" numberOfLines={4}>
            {profile.description}
          </Text>
        ) : null}
      </View>
      {serviceBadges.length > 0 && serviceDid ? (
        <View
          style={[
            px[3],
            pt[2],
            pb[2],
            borders.top.width.thin,
            { borderTopColor: theme.colors.border },
          ]}
        >
          {serviceBadges.map((badge, i) => (
            <BadgeRow
              key={i}
              badge={badge}
              serviceDid={serviceDid}
              streamer={streamer}
              issuerProfiles={profiles}
            />
          ))}
        </View>
      ) : null}
    </>
  );
};

// Web only: renders into document.body via a portal so FlatList re-renders and
// ancestor transforms/overflow can't affect it.
const ProfileCardOverlay = ({
  card,
  onClose,
  closeTimeoutRef,
}: {
  card: OpenCardData;
  onClose: () => void;
  closeTimeoutRef: React.MutableRefObject<ReturnType<typeof setTimeout> | null>;
}) => {
  const { theme } = useTheme();
  const data = useProfileCardData(card.author, card.badges);

  const cancelClose = useCallback(() => {
    if (closeTimeoutRef.current) {
      clearTimeout(closeTimeoutRef.current);
      closeTimeoutRef.current = null;
    }
  }, [closeTimeoutRef]);

  const scheduleClose = useCallback(() => {
    closeTimeoutRef.current = setTimeout(onClose, 150);
  }, [closeTimeoutRef, onClose]);

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
        onHoverIn={cancelClose}
        onHoverOut={scheduleClose}
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
  const closeTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const value = useMemo(
    () => ({ openCard, setOpenCard, closeTimeoutRef }),
    [openCard],
  );
  return (
    <OpenCardContext.Provider value={value}>
      {children}
      {openCard && Platform.OS === "web" && (
        <ProfileCardOverlay
          card={openCard}
          onClose={() => setOpenCard(null)}
          closeTimeoutRef={closeTimeoutRef}
        />
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

  // Native: self-contained DropdownMenu; rn-primitives renders it as a bottom sheet.
  // All hook data is resolved here (in the regular tree) and passed as plain props
  // so ProfileCardContent has no hooks to lose when inside the Modal.
  if (Platform.OS !== "web") {
    return (
      <DropdownMenu>
        <DropdownMenuTrigger ref={dropdownRef} asChild>
          <Pressable onPress={() => {}}>{children}</Pressable>
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
          marginLeft: -3,
          marginRight: -2,
          ...(hovered ? { backgroundColor: "rgba(255,255,255,0.15)" } : {}),
        },
      ]}
    >
      {children}
    </Pressable>
  );
};
