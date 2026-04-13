import {
  MenuContainer,
  MenuGroup,
  MenuSeparator,
  Text,
  useToast,
  zero,
} from "@streamplace/components";
import {
  Badge,
  BADGE_IMAGES,
} from "@streamplace/components/src/components/chat/badge";
import { borderRadius as radiusTokens } from "@streamplace/components/src/lib/theme/tokens";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { Image } from "expo-image";
import { Check } from "lucide-react-native";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  ActivityIndicator,
  ScrollView,
  TouchableOpacity,
  View,
} from "react-native";
import type { PlaceStreamBadgeDefs } from "streamplace";

const { gap, p, px, py, layout, w } = zero;

function BadgeIssuanceRow({
  badge,
  onToggle,
  toggling,
}: {
  badge: PlaceStreamBadgeDefs.BadgeIssuanceView;
  onToggle: (badge: PlaceStreamBadgeDefs.BadgeIssuanceView) => void;
  toggling: boolean;
}) {
  const { theme } = zero.useTheme();
  const isSelected = badge.selected ?? false;
  const badgeName = badge.name ?? badge.badgeType.split("#")[1];

  return (
    <TouchableOpacity
      onPress={() => onToggle(badge)}
      disabled={toggling}
      accessibilityRole="switch"
      accessibilityState={{ checked: isSelected }}
      accessibilityLabel={`${badgeName} badge`}
      accessibilityHint={isSelected ? "Deselect badge" : "Select badge"}
      style={[
        layout.flex.row,
        layout.flex.align.center,
        gap.all[3],
        py[3],
        px[4],
        { opacity: toggling ? 0.5 : 1 },
      ]}
    >
      {badge.imageUrl ? (
        <Image
          source={{ uri: badge.imageUrl }}
          style={{ width: 24, height: 24, borderRadius: radiusTokens.sm }}
        />
      ) : BADGE_IMAGES[badge.badgeType] ? (
        <Badge badgeType={badge.badgeType} size={24} />
      ) : (
        <View
          style={{
            width: 24,
            height: 24,
            borderRadius: radiusTokens.sm,
            backgroundColor: theme.colors.muted,
          }}
        />
      )}

      <View style={[{ flex: 1 }, gap.all[0.5]]}>
        <Text style={{ fontSize: 15, fontWeight: "500" }}>{badgeName}</Text>
        {badge.description && (
          <Text muted style={{ fontSize: 12 }} numberOfLines={2}>
            {badge.description}
          </Text>
        )}
        <Text muted style={{ fontSize: 11 }}>
          issued by {badge.issuer}
        </Text>
      </View>

      {toggling ? (
        <ActivityIndicator size="small" color={theme.colors.primary} />
      ) : isSelected ? (
        <View
          style={[
            {
              width: 22,
              height: 22,
              borderRadius: 11,
              backgroundColor: theme.colors.primary,
            },
            layout.flex.center,
          ]}
        >
          <Check size={13} color={theme.colors.primaryForeground} />
        </View>
      ) : (
        <View
          style={{
            width: 22,
            height: 22,
            borderRadius: 11,
            borderWidth: 1.5,
            borderColor: theme.colors.border,
          }}
        />
      )}
    </TouchableOpacity>
  );
}

export function BadgeSelectionManager() {
  const agent = usePDSAgent();
  const { theme } = zero.useTheme();
  const toast = useToast();

  const [loading, setLoading] = useState(true);
  const [streamerSlot, setStreamerSlot] =
    useState<PlaceStreamBadgeDefs.BadgeSlot | null>(null);
  const [userSlot, setUserSlot] =
    useState<PlaceStreamBadgeDefs.BadgeSlot | null>(null);
  const [toggling, setToggling] = useState<string | null>(null);
  const togglingRef = useRef<string | null>(null);

  const load = useCallback(async () => {
    if (!agent) return;
    try {
      setLoading(true);
      const res = await agent.place.stream.badge.getIssuedBadges({});
      setStreamerSlot(res.data.streamer);
      setUserSlot(res.data.user);
    } catch (e: any) {
      toast.show("Failed to load badges", e?.message, { variant: "error" });
    } finally {
      setLoading(false);
    }
  }, [agent]);

  useEffect(() => {
    load();
  }, [load]);

  const handleToggle = useCallback(
    async (badge: PlaceStreamBadgeDefs.BadgeIssuanceView) => {
      if (!agent?.did || togglingRef.current) return;
      togglingRef.current = badge.issuanceUri;
      setToggling(badge.issuanceUri);

      try {
        let currentRecord: Record<string, any> = {
          $type: "place.stream.chat.profile",
          selection: [],
          createdAt: new Date().toISOString(),
        };
        let swapCid: string | undefined;

        try {
          const getRes = await agent.com.atproto.repo.getRecord({
            repo: agent.did,
            collection: "place.stream.chat.profile",
            rkey: "self",
          });
          currentRecord = getRes.data.value as Record<string, any>;
          swapCid = getRes.data.cid;
        } catch {
          // no profile yet, will create
        }

        const currentSelection: Array<{ uri: string; cid: string }> =
          (currentRecord.selection as any[]) ?? [];

        let newSelection: Array<{ uri: string; cid: string }>;
        const isCurrentlySelected = badge.selected ?? false;

        if (isCurrentlySelected) {
          newSelection = currentSelection.filter(
            (s) => s.uri !== badge.issuanceUri,
          );
        } else {
          const ref = { uri: badge.issuanceUri, cid: "" };
          newSelection = [
            ...currentSelection.filter((s) => s.uri !== badge.issuanceUri),
            ref,
          ];
        }

        await agent.com.atproto.repo.putRecord({
          repo: agent.did,
          collection: "place.stream.chat.profile",
          rkey: "self",
          record: { ...currentRecord, selection: newSelection },
          swapRecord: swapCid,
        });

        await load();
      } catch (e: any) {
        toast.show("Failed to update badge selection", e?.message, {
          variant: "error",
        });
      } finally {
        togglingRef.current = null;
        setToggling(null);
      }
    },
    [agent, load],
  );

  if (loading) {
    return (
      <View style={[{ flex: 1 }, layout.flex.center]}>
        <ActivityIndicator color={theme.colors.primary} />
      </View>
    );
  }

  const hasStreamerBadges = (streamerSlot?.available?.length ?? 0) > 0;
  const hasUserBadges = (userSlot?.available?.length ?? 0) > 0;

  if (!hasStreamerBadges && !hasUserBadges) {
    return (
      <ScrollView
        contentContainerStyle={[
          p[4],
          gap.all[3],
          layout.flex.align.center,
          { paddingTop: 48 },
        ]}
      >
        <Text muted center>
          No badges yet. Badges appear here when streamers or the server issues
          them to you.
        </Text>
      </ScrollView>
    );
  }

  return (
    <ScrollView contentContainerStyle={[p[2]]}>
      <View style={{ maxWidth: 500, width: "100%", alignSelf: "center" }}>
        <MenuContainer>
          {hasStreamerBadges && (
            <MenuGroup>
              <View style={[px[4], py[3]]}>
                <Text
                  accessibilityRole="header"
                  muted
                  uppercase
                  style={{
                    fontSize: 12,
                    fontWeight: "600",
                    letterSpacing: 0.5,
                  }}
                >
                  Streamer badges
                </Text>
              </View>
              {streamerSlot!.available.map((badge, i) => (
                <View key={badge.issuanceUri}>
                  {i > 0 && <MenuSeparator />}
                  <BadgeIssuanceRow
                    badge={badge}
                    onToggle={handleToggle}
                    toggling={toggling === badge.issuanceUri}
                  />
                </View>
              ))}
            </MenuGroup>
          )}

          {hasUserBadges && (
            <MenuGroup>
              <View style={[px[4], py[3]]}>
                <Text
                  accessibilityRole="header"
                  muted
                  uppercase
                  style={{
                    fontSize: 12,
                    fontWeight: "600",
                    letterSpacing: 0.5,
                  }}
                >
                  Cosmetic badges
                </Text>
              </View>
              {userSlot!.available.map((badge, i) => (
                <View key={badge.issuanceUri}>
                  {i > 0 && <MenuSeparator />}
                  <BadgeIssuanceRow
                    badge={badge}
                    onToggle={handleToggle}
                    toggling={toggling === badge.issuanceUri}
                  />
                </View>
              ))}
            </MenuGroup>
          )}
        </MenuContainer>
      </View>
    </ScrollView>
  );
}
