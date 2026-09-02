import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
  formatHandleWithAt,
  hexToRgba,
  ResponsiveDropdownMenuContent,
  Text,
  useBadgeSlots,
  useLivestream,
  useSetBadgeSlots,
  useToast,
  zero,
} from "@streamplace/components";
import {
  Badge,
  BADGE_IMAGES,
} from "@streamplace/components/src/components/chat/badge";
import { borderRadius as radiusTokens } from "@streamplace/components/src/lib/theme/tokens";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { p } from "@streamplace/components/src/ui";
import { useNameColorPicker } from "components/name-color-picker/name-color-picker";
import { Image } from "expo-image";
import { Check, SwatchBook } from "lucide-react-native";
import { useCallback, useEffect, useRef, useState } from "react";
import { ActivityIndicator, TouchableOpacity, View } from "react-native";
import { useStore } from "store";
import { useUserProfile } from "store/hooks";
import { place } from "streamplace";

const { gap, layout } = zero;

const HOT_COLORS = [
  { r: 232, g: 121, b: 160 }, // rose
  { r: 251, g: 113, b: 133 }, // coral
  { r: 251, g: 146, b: 60 }, // orange
  { r: 251, g: 191, b: 36 }, // amber
  { r: 163, g: 230, b: 53 }, // lime
  { r: 45, g: 212, b: 191 }, // teal
  { r: 56, g: 189, b: 248 }, // sky
  { r: 167, g: 139, b: 250 }, // lavender
  { r: 244, g: 114, b: 182 }, // pink
  { r: 253, g: 224, b: 71 }, // yellow
  { r: 52, g: 211, b: 153 }, // emerald
  { r: 96, g: 165, b: 250 }, // blue
];

export function BadgePicker() {
  const agent = usePDSAgent();
  const me = useUserProfile();
  const { theme } = zero.useTheme();
  const toast = useToast();
  const linfo = useLivestream();
  const streamerDid = linfo?.author?.did;

  const badgeSlots = useBadgeSlots();
  const setBadgeSlots = useSetBadgeSlots();
  const [loading, setLoading] = useState(false);
  const [toggling, setToggling] = useState<string | null>(null);
  const togglingRef = useRef<string | null>(null);

  const streamerSlot = badgeSlots?.streamer ?? null;
  const userSlot = badgeSlots?.user ?? null;
  const activeBadge = streamerSlot?.selected ?? userSlot?.selected ?? null;

  const load = useCallback(async () => {
    if (!agent?.did || badgeSlots !== null) return;
    try {
      setLoading(true);
      const res = await agent.client.call(place.stream.badge.getIssuedBadges, {
        streamer: streamerDid as any,
      });
      setBadgeSlots({ streamer: res.streamer, user: res.user });
    } catch (e: any) {
      toast.show("Failed to load badges", e?.message, { variant: "error" });
    } finally {
      setLoading(false);
    }
  }, [agent, streamerDid, badgeSlots, setBadgeSlots]);

  useEffect(() => {
    load();
  }, [load]);

  const handleToggle = useCallback(
    async (
      badge: place.stream.badge.defs.BadgeIssuanceView,
      slot: "streamer" | "global",
    ) => {
      if (!agent?.did || togglingRef.current) return;
      togglingRef.current = badge.issuanceUri;
      setToggling(badge.issuanceUri);

      try {
        let currentRecord: Record<string, any> = {
          $type: "place.stream.chat.profile",
        };
        let swapCid: string | undefined;

        try {
          const getRes = await agent.client.get(place.stream.chat.profile);
          currentRecord = getRes.value as Record<string, any>;
          swapCid = getRes.cid;
        } catch {
          // no profile yet, will create
        }

        const currentBadges: Record<string, any> =
          (currentRecord.badges as Record<string, any>) ?? {};
        const isCurrentlySelected = badge.selected ?? false;
        const ref = { uri: badge.issuanceUri, cid: badge.issuanceCid ?? "" };

        let newBadges: Record<string, any>;
        if (slot === "streamer") {
          const currentStreamer: Array<{
            streamer: string;
            badge: { uri: string; cid: string };
          }> = (currentBadges.streamer as any[]) ?? [];
          newBadges = {
            ...currentBadges,
            streamer: isCurrentlySelected
              ? currentStreamer.filter((s) => s.badge.uri !== badge.issuanceUri)
              : [
                  ...currentStreamer.filter((s) => s.streamer !== badge.issuer),
                  { streamer: badge.issuer, badge: ref },
                ],
          };
        } else {
          newBadges = {
            ...currentBadges,
            global: isCurrentlySelected ? undefined : ref,
          };
        }

        const { $type: _omitType, ...currentRecordRest } = currentRecord;
        await agent.client.put(
          place.stream.chat.profile,
          { ...currentRecordRest, badges: newBadges } as any,
          { swapRecord: swapCid },
        );

        if (slot === "streamer") {
          setBadgeSlots({
            streamer: (() => {
              const prev = streamerSlot;
              if (!prev) return prev;
              const available = prev.available.map((b) =>
                b.issuanceUri === badge.issuanceUri
                  ? { ...b, selected: !isCurrentlySelected }
                  : b.issuer === badge.issuer
                    ? { ...b, selected: false }
                    : b,
              );
              return {
                available,
                selected: isCurrentlySelected
                  ? undefined
                  : available.find((b) => b.issuanceUri === badge.issuanceUri),
              };
            })(),
            user: userSlot,
          });
        } else {
          setBadgeSlots({
            streamer: streamerSlot,
            user: (() => {
              const prev = userSlot;
              if (!prev) return prev;
              const available = prev.available.map((b) =>
                b.issuanceUri === badge.issuanceUri
                  ? { ...b, selected: !isCurrentlySelected }
                  : { ...b, selected: false },
              );
              return {
                available,
                selected: isCurrentlySelected
                  ? undefined
                  : available.find((b) => b.issuanceUri === badge.issuanceUri),
              };
            })(),
          });
        }
      } catch (e: any) {
        toast.show("Failed to update badge", e?.message, { variant: "error" });
      } finally {
        togglingRef.current = null;
        setToggling(null);
      }
    },
    [agent, streamerSlot, userSlot, setBadgeSlots],
  );

  const { currentColor, openModal, modal } = useNameColorPicker();
  const createChatProfileRecord = useStore((s) => s.createChatProfileRecord);

  if (!agent?.did) return null;

  const hasStreamerBadges = (streamerSlot?.available?.length ?? 0) > 0;
  const hasUserBadges = (userSlot?.available?.length ?? 0) > 0;
  const streamerName = linfo?.author;

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger>
          <BadgeTriggerButton badge={activeBadge} loading={loading} />
        </DropdownMenuTrigger>
        <ResponsiveDropdownMenuContent align="start">
          <View style={[gap.all[1]]}>
            <Text size="lg" style={[zero.pl[2], zero.pb[1], zero.pt[1]]}>
              Your chat appearance
            </Text>
            <DropdownMenuGroup>
              <View
                style={[
                  layout.flex.row,
                  layout.flex.alignCenter,
                  gap.all[1],
                  p[2],
                ]}
              >
                <Text>
                  <Text style={{ color: currentColor }}>
                    <View>
                      {streamerSlot?.selected && (
                        <Badge
                          isInChat={true}
                          badgeType={streamerSlot.selected.badgeType}
                          imageUrl={streamerSlot.selected.imageUrl}
                          size={22}
                        />
                      )}
                      {userSlot?.selected && (
                        <Badge
                          isInChat={true}
                          badgeType={userSlot.selected.badgeType}
                          imageUrl={userSlot.selected.imageUrl}
                          size={22}
                        />
                      )}
                    </View>
                    @{me?.handle || me?.did || "yourhand.le"}
                  </Text>
                  : what is a bluesky account
                </Text>
              </View>
            </DropdownMenuGroup>

            {loading ? (
              <View style={[layout.flex.center, { height: 60 }]}>
                <ActivityIndicator color={theme.colors.primary} />
              </View>
            ) : hasStreamerBadges || hasUserBadges ? (
              <>
                {hasStreamerBadges && (
                  <DropdownMenuGroup
                    title={`${streamerName ? formatHandleWithAt(streamerName) + " " : ""}badges`}
                    description={`Shows up only in ${streamerName ? `${formatHandleWithAt(streamerName)}'s` : "this"} channel`}
                  >
                    {streamerSlot!.available.map((badge) => (
                      <BadgeCheckboxItem
                        key={badge.issuanceUri}
                        badge={badge}
                        toggling={toggling === badge.issuanceUri}
                        onToggle={() => handleToggle(badge, "streamer")}
                      />
                    ))}
                  </DropdownMenuGroup>
                )}
                {hasUserBadges && (
                  <DropdownMenuGroup
                    title="Global badge"
                    description="Shows up across all channels"
                  >
                    {userSlot!.available.map((badge) => (
                      <BadgeCheckboxItem
                        key={badge.issuanceUri}
                        badge={badge}
                        toggling={toggling === badge.issuanceUri}
                        onToggle={() => handleToggle(badge, "global")}
                      />
                    ))}
                  </DropdownMenuGroup>
                )}
              </>
            ) : null}
            <DropdownMenuGroup
              title="Handle color"
              description="Change your color in chat"
            >
              <View
                style={[
                  layout.flex.row,
                  { flexWrap: "wrap", gap: 8, padding: 6, paddingTop: 10 },
                ]}
              >
                {HOT_COLORS.map(({ r, g, b }) => {
                  const hex = `rgb(${r}, ${g}, ${b})`; // token-ok: dynamic swatch
                  const isSelected = currentColor === hex;
                  return (
                    <TouchableOpacity
                      key={hex}
                      onPress={() => createChatProfileRecord(r, g, b)}
                      style={{
                        width: 28,
                        height: 28,
                        borderRadius: 999,
                        backgroundColor: hex,
                        borderWidth: isSelected ? 2 : 0,
                        borderColor: "white",
                        shadowColor: isSelected ? hex : "transparent",
                        shadowOpacity: isSelected ? 0.8 : 0,
                        shadowRadius: 4,
                        shadowOffset: { width: 0, height: 0 },
                        elevation: isSelected ? 4 : 0,
                      }}
                    />
                  );
                })}
              </View>
              <DropdownMenuItem onPress={openModal}>
                <View
                  style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
                >
                  <SwatchBook size={20} color={currentColor} />
                  <Text style={{ color: currentColor }}>Custom color...</Text>
                </View>
              </DropdownMenuItem>
            </DropdownMenuGroup>
          </View>
        </ResponsiveDropdownMenuContent>
      </DropdownMenu>
      {modal}
    </>
  );
}

function BadgeTriggerButton({
  badge,
  loading,
}: {
  badge: place.stream.badge.defs.BadgeIssuanceView | null;
  loading: boolean;
}) {
  const { theme } = zero.useTheme();
  const [hover, setHover] = useState(false);

  return (
    <View
      style={{
        paddingHorizontal: 9.5,
        marginRight: -9.5,
        height: 43,
        borderRadius: radiusTokens.xl,
        alignItems: "center",
        justifyContent: "center",
        backgroundColor: hover
          ? theme.colors.muted
          : hexToRgba(theme.colors.muted, 0.3),
      }}
      onPointerEnter={() => setHover(true)}
      onPointerLeave={() => setHover(false)}
    >
      {loading ? (
        <ActivityIndicator size="small" color={theme.colors.mutedForeground} />
      ) : badge?.imageUrl ? (
        <Image
          source={{ uri: badge.imageUrl }}
          style={{ width: 24, height: 24, borderRadius: 5 }}
        />
      ) : badge && BADGE_IMAGES[badge.badgeType] ? (
        <Badge badgeType={badge.badgeType} size={22} />
      ) : (
        <View
          style={{
            width: 22,
            height: 22,
            borderRadius: 4,
            backgroundColor: theme.colors.mutedForeground,
            opacity: 0.3,
          }}
        />
      )}
    </View>
  );
}

function BadgeCheckboxItem({
  badge,
  toggling,
  onToggle,
}: {
  badge: place.stream.badge.defs.BadgeIssuanceView;
  toggling: boolean;
  onToggle: () => void;
}) {
  const { theme } = zero.useTheme();
  const badgeName = badge.name ?? badge.badgeType.split("#")[1];
  const desc = badge.description;

  return (
    <DropdownMenuCheckboxItem
      checked={badge.selected ?? false}
      onCheckedChange={onToggle}
      closeOnPress={false}
      disabled={toggling}
    >
      <View
        style={[
          layout.flex.row,
          layout.flex.alignCenter,
          gap.all[2],
          { opacity: toggling ? 0.5 : 1 },
        ]}
      >
        {badge.imageUrl ? (
          <Image
            source={{ uri: badge.imageUrl }}
            style={{ width: 22, height: 22, borderRadius: 3 }}
          />
        ) : BADGE_IMAGES[badge.badgeType] ? (
          <Badge badgeType={badge.badgeType} size={20} />
        ) : (
          <View
            style={[
              {
                width: 22,
                height: 22,
                borderRadius: 3,
                backgroundColor: theme.colors.muted,
              },
            ]}
          />
        )}
        <View style={[zero.flex.grow[1]]}>
          <Text size="sm">{badgeName}</Text>
          {desc && (
            <Text muted size="xs">
              {desc}
            </Text>
          )}
        </View>
        {toggling ? (
          <ActivityIndicator size="small" color={theme.colors.primary} />
        ) : badge.selected ? (
          <Check size={14} strokeWidth={3} color={theme.colors.foreground} />
        ) : null}
      </View>
    </DropdownMenuCheckboxItem>
  );
}
