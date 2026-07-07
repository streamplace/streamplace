import {
  MenuContainer,
  MenuGroup,
  MenuLabel,
  MenuSeparator,
  Text,
  useToast,
  useTranslation,
  zero,
} from "@streamplace/components";
import {
  Badge,
  BADGE_IMAGES,
} from "@streamplace/components/src/components/chat/badge";
import { borderRadius as radiusTokens } from "@streamplace/components/src/lib/theme/tokens";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { flex, mt, pt } from "@streamplace/components/src/ui";
import { Image } from "expo-image";
import { Check } from "lucide-react-native";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  ActivityIndicator,
  ScrollView,
  TouchableOpacity,
  View,
} from "react-native";
import { place } from "streamplace";

const { gap, p, px, py, layout, w } = zero;

function BadgeIssuanceRow({
  badge,
  onToggle,
  toggling,
}: {
  badge: place.stream.badge.defs.BadgeIssuanceView;
  onToggle: (badge: place.stream.badge.defs.BadgeIssuanceView) => void;
  toggling: boolean;
}) {
  const { theme } = zero.useTheme();
  const { t } = useTranslation("settings");
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

      <View style={[flex.grow[1]]}>
        <Text size="sm" leading="tight">
          {badgeName}
        </Text>
        {badge.description && (
          <Text muted size="xs" numberOfLines={2} leading="snug">
            {badge.description}
          </Text>
        )}
        <Text muted size="xs">
          {t("badges-issued-by", { issuer: badge.issuer })}
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
  const { t } = useTranslation("settings");

  const [loading, setLoading] = useState(true);
  const [streamerSlot, setStreamerSlot] =
    useState<place.stream.badge.defs.BadgeSlot | null>(null);
  const [userSlot, setUserSlot] =
    useState<place.stream.badge.defs.BadgeSlot | null>(null);
  const [toggling, setToggling] = useState<string | null>(null);
  const togglingRef = useRef<string | null>(null);

  const load = useCallback(async () => {
    if (!agent) return;
    try {
      setLoading(true);
      const res = await agent.client.call(
        place.stream.badge.getIssuedBadges,
        {},
      );
      setStreamerSlot(res.streamer);
      setUserSlot(res.user);
    } catch (e: any) {
      toast.show(t("badges-failed-load"), e?.message, { variant: "error" });
    } finally {
      setLoading(false);
    }
  }, [agent]);

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

        // Optimistically update local state
        if (slot === "streamer") {
          setStreamerSlot((prev) => {
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
          });
        } else {
          setUserSlot((prev) => {
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
          });
        }
      } catch (e: any) {
        toast.show(t("badges-failed-update"), e?.message, {
          variant: "error",
        });
      } finally {
        togglingRef.current = null;
        setToggling(null);
      }
    },
    [agent],
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
          mt[12],
          gap.all[3],
          layout.flex.align.center,
          { paddingTop: 48 },
        ]}
      >
        <Text muted center>
          {t("badges-empty-state")}
        </Text>
      </ScrollView>
    );
  }

  return (
    <ScrollView contentContainerStyle={[p[2], pt[28]]}>
      <View style={{ maxWidth: 500, width: "100%", alignSelf: "center" }}>
        <MenuContainer>
          {hasStreamerBadges && (
            <>
              <MenuLabel>{t("badges-streamer-section")}</MenuLabel>
              <MenuGroup>
                {streamerSlot!.available.map((badge, i) => (
                  <View key={badge.issuanceUri}>
                    {i > 0 && <MenuSeparator />}
                    <BadgeIssuanceRow
                      badge={badge}
                      onToggle={(b) => handleToggle(b, "streamer")}
                      toggling={toggling === badge.issuanceUri}
                    />
                  </View>
                ))}
              </MenuGroup>
            </>
          )}

          {hasUserBadges && (
            <>
              <MenuLabel>{t("badges-cosmetic-section")}</MenuLabel>
              <MenuGroup>
                {userSlot!.available.map((badge, i) => (
                  <View key={badge.issuanceUri}>
                    {i > 0 && <MenuSeparator />}
                    <BadgeIssuanceRow
                      badge={badge}
                      onToggle={(b) => handleToggle(b, "global")}
                      toggling={toggling === badge.issuanceUri}
                    />
                  </View>
                ))}
              </MenuGroup>
            </>
          )}
        </MenuContainer>
      </View>
    </ScrollView>
  );
}
