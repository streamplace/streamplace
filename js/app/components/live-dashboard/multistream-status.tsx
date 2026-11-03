import { useNavigation } from "@react-navigation/native";
import {
  Button,
  Loader,
  Switch,
  Text,
  View,
  zero,
} from "@streamplace/components";
import {
  borderAlphas,
  surfaces,
  textAlphas,
} from "@streamplace/components/src/lib/theme/tokens";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { Plus } from "lucide-react-native";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Platform } from "react-native";
import Animated, {
  cancelAnimation,
  useAnimatedStyle,
  useSharedValue,
  withRepeat,
  withTiming,
} from "react-native-reanimated";
import { getStreamplaceNavigationAction } from "src/linking-config";
import { place } from "streamplace";

const { flex, p, gap, layout, bg, borders, text, r } = zero;

interface MultistreamTargetViewHydrated
  extends place.stream.multistream.defs.TargetView {
  record: any;
}

export default function MultistreamStatus() {
  const agent = usePDSAgent();
  const navigation = useNavigation();
  const { t } = useTranslation("settings");
  const [targets, setTargets] = useState<MultistreamTargetViewHydrated[]>([]);
  const [loading, setLoading] = useState(true);
  const [listError, setListError] = useState(false);
  const [toggleErrors, setToggleErrors] = useState<Set<string>>(new Set());
  const [togglingTargets, setTogglingTargets] = useState<Set<string>>(
    new Set(),
  );

  // Reanimated animation for connecting states
  const opacity = useSharedValue(1);

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: opacity.value,
  }));

  useEffect(() => {
    const hasConnectingTargets = targets.some(
      (t) => t.record.active && t.latestEvent?.status === "pending",
    );

    if (hasConnectingTargets) {
      opacity.value = withRepeat(withTiming(0.3, { duration: 1000 }), -1, true);
    } else {
      cancelAnimation(opacity);
      opacity.value = withTiming(1, { duration: 200 });
    }
  }, [targets, opacity]);

  const loadTargets = useCallback(async () => {
    if (!agent) return;

    try {
      setLoading(true);
      const targetViews = await agent.client.call(
        place.stream.multistream.listTargets,
        { limit: 50 },
      );
      setTargets(targetViews.targets as MultistreamTargetViewHydrated[]);
      setListError(false);
    } catch (error) {
      if (Platform.OS === "web") {
        setListError(true);
      } else {
        console.error("Failed to load multistream targets:", error);
        setTargets([]);
      }
    } finally {
      setLoading(false);
    }
  }, [agent]);

  const toggleTarget = useCallback(
    async (target: MultistreamTargetViewHydrated, newActiveState: boolean) => {
      if (!agent) return;
      try {
        setTogglingTargets((prev) => new Set(prev).add(target.uri));
        if (Platform.OS === "web") {
          setToggleErrors((prev) => {
            const next = new Set(prev);
            next.delete(target.uri);
            return next;
          });
        }
        const updated = await agent.client.call(
          place.stream.multistream.putTarget,
          {
            multistreamTarget: {
              ...target.record,
              active: newActiveState,
            },
            rkey: target.uri.split("/").pop() || "",
          },
        );
        if (Platform.OS === "web") {
          setTargets((previous) =>
            previous.map((item) =>
              item.uri === target.uri
                ? (updated as MultistreamTargetViewHydrated)
                : item,
            ),
          );
        } else {
          await loadTargets();
        }
      } catch (error) {
        if (Platform.OS === "web") {
          setToggleErrors((previous) => new Set(previous).add(target.uri));
        } else {
          console.error("Failed to toggle multistream target:", error);
        }
      } finally {
        setTogglingTargets((prev) => {
          const newSet = new Set(prev);
          newSet.delete(target.uri);
          return newSet;
        });
      }
    },
    [agent, loadTargets],
  );

  useEffect(() => {
    loadTargets();
  }, [loadTargets]);

  const getTargetName = (target: MultistreamTargetViewHydrated) => {
    if (target.record.name) return target.record.name;
    if (target.record.url) {
      try {
        const u = new URL(target.record.url);
        return u.host;
      } catch {
        return "Untitled Target";
      }
    }
    return "Untitled Target";
  };

  const getTargetHostname = (target: MultistreamTargetViewHydrated) => {
    if (!target.record.url) return null;
    try {
      const u = new URL(target.record.url);
      return u.host.split(":")[0];
    } catch {
      return null;
    }
  };

  const getStatusColor = (target: MultistreamTargetViewHydrated) => {
    if (!target.record.active) return text.gray[500];

    switch (target.latestEvent?.status) {
      case "active":
        return text.green[400];
      case "error":
        return text.red[400];
      case "pending":
        return text.yellow[400];
      default:
        return text.gray[600];
    }
  };

  return (
    <View
      testID="live-stream-destinations"
      style={[
        { backgroundColor: surfaces.dark[1] },
        r.lg,
        borders.width.thin,
        { borderColor: borderAlphas.dark.strong },
      ]}
    >
      {listError && (
        <View style={[p[4], gap.all[2], { alignItems: "flex-start" }]}>
          <Text color="destructive">Could not load destinations</Text>
          <Button
            size="sm"
            width="min"
            variant="secondary"
            onPress={loadTargets}
            disabled={loading || togglingTargets.size > 0}
          >
            Retry
          </Button>
        </View>
      )}
      {loading && targets.length === 0 ? (
        <View style={[p[4]]}>
          <Loader size="small" />
        </View>
      ) : targets.length === 0 && !listError ? (
        <View
          style={[
            Platform.OS === "web" ? p[3] : p[4],
            Platform.OS === "web" ? gap.all[2] : { gap: 12 },
            { alignItems: "flex-start" },
          ]}
        >
          <View style={{ gap: 2 }}>
            <Text
              style={{
                color: textAlphas.dark[2],
                fontSize: 13,
                fontWeight: "500",
              }}
            >
              {t("no-multistream-destinations")}
            </Text>
            {Platform.OS !== "web" && (
              <Text style={{ color: textAlphas.dark[3], fontSize: 12 }}>
                Restream to Twitch, YouTube, and more.
              </Text>
            )}
          </View>
          <Button
            size="sm"
            width="min"
            variant="secondary"
            leftIcon={<Plus size={14} />}
            onPress={() => {
              if (Platform.OS === "web") {
                const action = getStreamplaceNavigationAction(
                  "/settings/streaming/multistream",
                );
                if (action) navigation.dispatch(action);
              } else {
                (navigation as any).navigate("SettingsTab", {
                  screen: "MultistreamCategory",
                });
              }
            }}
          >
            Add destination
          </Button>
        </View>
      ) : targets.length > 0 ? (
        <View style={[p[3], gap.all[2]]}>
          {targets.map((target) => (
            <View
              key={target.uri}
              style={[
                layout.flex.row,
                layout.flex.alignCenter,
                layout.flex.spaceBetween,
                Platform.OS === "web" ? gap.all[2] : {},
                p[2],
                { backgroundColor: surfaces.dark[2] },
                r.md,
                borders.width.thin,
                { borderColor: borderAlphas.dark.strong },
              ]}
            >
              <View
                style={[
                  flex.values[1],
                  Platform.OS === "web" ? { minWidth: 0 } : {},
                ]}
              >
                <View
                  style={
                    Platform.OS === "web"
                      ? [layout.flex.column]
                      : [layout.flex.row, layout.flex.alignCenter, gap.all[2]]
                  }
                >
                  <Text numberOfLines={Platform.OS === "web" ? 1 : undefined}>
                    {getTargetName(target)}
                  </Text>
                  {target.record.name && getTargetHostname(target) && (
                    <Text
                      color="muted"
                      numberOfLines={Platform.OS === "web" ? 1 : undefined}
                    >
                      {getTargetHostname(target)}
                    </Text>
                  )}
                </View>
                {(target.latestEvent ||
                  (Platform.OS === "web" && !target.record.active)) && (
                  <Animated.Text
                    style={[
                      getStatusColor(target),
                      { fontSize: 11 },
                      (Platform.OS !== "web" || target.record.active) &&
                        target.latestEvent?.status === "pending" &&
                        animatedStyle,
                    ]}
                  >
                    {Platform.OS === "web" && !target.record.active
                      ? t("inactive")
                      : target.latestEvent?.status}
                  </Animated.Text>
                )}
                {toggleErrors.has(target.uri) && (
                  <Text color="destructive">Could not update destination</Text>
                )}
              </View>
              <Switch
                value={target.record.active}
                accessibilityLabel={
                  Platform.OS === "web" ? getTargetName(target) : undefined
                }
                onValueChange={(active) => toggleTarget(target, active)}
                disabled={
                  togglingTargets.has(target.uri) ||
                  (Platform.OS === "web" && loading)
                }
              />
            </View>
          ))}
        </View>
      ) : null}
    </View>
  );
}
