import { ProfileViewBasic } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { TriggerRef } from "@rn-primitives/dropdown-menu";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Image, Platform, Pressable, View } from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { useAvatars } from "../../hooks/useAvatars";
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
    issuedBy: "{issuer} p.p. {streamer}",
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

interface OpenCardContextValue {
  openUri: string | null;
  setOpenUri: (uri: string | null) => void;
}

const OpenCardContext = createContext<OpenCardContextValue>({
  openUri: null,
  setOpenUri: () => {},
});

export const ProfileCardProvider = ({
  children,
}: {
  children: React.ReactNode;
}) => {
  const [openUri, setOpenUri] = useState<string | null>(null);
  return (
    <OpenCardContext.Provider value={{ openUri, setOpenUri }}>
      {children}
    </OpenCardContext.Provider>
  );
};

const BadgeRow = ({
  badge,
  serviceDid,
}: {
  badge: NonNullable<ChatMessageViewHydrated["badges"]>[number];
  serviceDid: string;
}) => {
  const streamer = useLivestreamStore((x) => x.livestream?.author);
  const isServiceIssued = badge.issuer === serviceDid;
  const issuerDids = useMemo(
    () => (isServiceIssued ? [] : [badge.issuer]),
    [isServiceIssued, badge.issuer],
  );
  const issuerProfiles = useAvatars(issuerDids);
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
      style={{
        flexDirection: "row",
        alignItems: "center",
        gap: 12,
        paddingLeft: 6,
      }}
    >
      <Badge badgeType={badge.badgeType} size={32} />
      <View style={{ flex: 1, gap: 2 }}>
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
  const nodeUrl = useUrl();
  const serviceDid = nodeUrl
    ? `did:web:${nodeUrl.replace(/^https?:\/\//, "")}`
    : null;

  const { openUri, setOpenUri } = useContext(OpenCardContext);
  const isOpen = openUri === uri;
  const thisRef = useRef<TriggerRef>(null);
  const [hovered, setHovered] = useState(false);

  const profiles = useAvatars(author.did ? [author.did] : []);
  const profile = profiles[author.did];

  useEffect(() => {
    isOpen ? thisRef.current?.open() : thisRef.current?.close();
  }, [isOpen]);

  const onOpenChange = useCallback(
    (open: boolean) => {
      setOpenUri(open ? uri : null);
    },
    [uri, setOpenUri],
  );

  const serviceBadges = useMemo(
    () => badges?.filter((b) => serviceDid && b.issuer === serviceDid) ?? [],
    [badges, serviceDid],
  );

  return (
    <DropdownMenu onOpenChange={onOpenChange}>
      <DropdownMenuTrigger ref={thisRef} asChild>
        <Pressable
          onPress={() => {}}
          {...(Platform.OS === "web"
            ? {
                onHoverIn: () => setHovered(true),
                onHoverOut: () => setHovered(false),
              }
            : {})}
          style={{
            paddingHorizontal: 3,
            flexDirection: "row",
            gap: 4,
            marginLeft: -3,
            paddingLeft: 3,
            marginRight: -2,
            ...(Platform.OS === "web" && hovered
              ? { backgroundColor: "rgba(255,255,255,0.15)", borderRadius: 6 }
              : {}),
          }}
        >
          {children}
        </Pressable>
      </DropdownMenuTrigger>
      <DropdownMenuContent style={{ minWidth: 280, maxWidth: 320 }}>
        <View>
          {profile?.banner ? (
            <Image
              source={{ uri: profile.banner }}
              style={{
                width: "100%",
                height: 80,
                borderRadius: theme.borderRadius.md,
              }}
            />
          ) : (
            <View
              style={{
                width: "100%",
                height: 80,
                borderRadius: theme.borderRadius.md,
                backgroundColor: theme.colors.muted,
              }}
            />
          )}
          <View
            style={{
              flexDirection: "row",
              alignItems: "flex-end",
              marginTop: -24,
              paddingHorizontal: 12,
            }}
          >
            {profile?.avatar ? (
              <Image
                source={{ uri: profile.avatar }}
                style={{
                  width: 48,
                  height: 48,
                  borderRadius: 24,
                  borderWidth: 2,
                  borderColor: theme.colors.card,
                }}
              />
            ) : (
              <View
                style={{
                  width: 48,
                  height: 48,
                  borderRadius: 24,
                  backgroundColor: theme.colors.mutedForeground,
                  borderWidth: 2,
                  borderColor: theme.colors.card,
                }}
              />
            )}
          </View>
          <View style={{ paddingHorizontal: 12 }}>
            <Text>@{author.handle}</Text>
            {profile?.description ? (
              <Text size="sm" color="muted" numberOfLines={4}>
                {profile.description}
              </Text>
            ) : null}
          </View>
          {serviceBadges.length > 0 && serviceDid ? (
            <View
              style={{
                marginTop: 12,
                paddingHorizontal: 12,
                paddingBottom: 8,
                borderTopWidth: 1,
                borderTopColor: theme.colors.border,
                paddingTop: 8,
              }}
            >
              {serviceBadges.map((badge, i) => (
                <BadgeRow key={i} badge={badge} serviceDid={serviceDid} />
              ))}
            </View>
          ) : null}
        </View>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
