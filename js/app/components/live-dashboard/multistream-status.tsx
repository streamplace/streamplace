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
import Animated, {
  cancelAnimation,
  useAnimatedStyle,
  useSharedValue,
  withRepeat,
  withTiming,
} from "react-native-reanimated";
import { place } from "streamplace";

const { flex, p, gap, layout, bg, borders, text, r } = zero;

interface MultistreamTargetViewHydrated
  extends place.stream.multistream.defs.TargetView {
  record: any;
}

export default function MultistreamStatus() {
  const agent = usePDSAgent();
  const navigation = useNavigation();
  const [targets, setTargets] = useState<MultistreamTargetViewHydrated[]>([]);
  const [loading, setLoading] = useState(true);
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
    } catch (error) {
      console.error("Failed to load multistream targets:", error);
      setTargets([]);
    } finally {
      setLoading(false);
    }
  }, [agent]);

  const toggleTarget = useCallback(
    async (target: MultistreamTargetViewHydrated, newActiveState: boolean) => {
      if (!agent) return;
      try {
        setTogglingTargets((prev) => new Set(prev).add(target.uri));
        await agent.client.call(place.stream.multistream.putTarget, {
          multistreamTarget: {
            ...target.record,
            active: newActiveState,
          },
          rkey: target.uri.split("/").pop() || "",
        });
        await loadTargets();
      } catch (error) {
        console.error("Failed to toggle multistream target:", error);
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

  const activeTargets = targets.filter((t) => t.record.active);
  const inactiveTargets = targets.filter((t) => !t.record.active);

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
      style={[
        { backgroundColor: surfaces.dark[1] },
        r.lg,
        borders.width.thin,
        { borderColor: borderAlphas.dark.strong },
      ]}
    >
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          p[4],
          borders.bottom.width.thin,
          { borderBottomColor: borderAlphas.dark.strong },
        ]}
      >
        <Text style={[text.white, { fontSize: 15, fontWeight: "600" }]}>
          Multistream
        </Text>
        {loading ? (
          <Loader size="small" />
        ) : targets.length > 0 ? (
          <Text
            style={{
              color: textAlphas.dark[3],
              fontSize: 12,
              fontWeight: "600",
            }}
          >
            {targets.length}
          </Text>
        ) : null}
      </View>
      {targets.length === 0 ? (
        <View style={[p[4], { gap: 12, alignItems: "flex-start" }]}>
          <View style={{ gap: 2 }}>
            <Text
              style={{
                color: textAlphas.dark[2],
                fontSize: 13,
                fontWeight: "500",
              }}
            >
              No destinations yet
            </Text>
            <Text style={{ color: textAlphas.dark[3], fontSize: 12 }}>
              Restream to Twitch, YouTube, and more.
            </Text>
          </View>
          <Button
            size="sm"
            width="min"
            variant="secondary"
            leftIcon={<Plus size={14} />}
            onPress={() =>
              (navigation as any).navigate("SettingsTab", {
                screen: "MultistreamCategory",
              })
            }
          >
            Add destination
          </Button>
        </View>
      ) : (
        <View style={[p[3], gap.all[2]]}>
          {targets.map((target) => (
            <View
              key={target.uri}
              style={[
                layout.flex.row,
                layout.flex.alignCenter,
                layout.flex.spaceBetween,
                p[2],
                { backgroundColor: surfaces.dark[2] },
                r.md,
                borders.width.thin,
                { borderColor: borderAlphas.dark.strong },
              ]}
            >
              <View style={[flex.values[1]]}>
                <View
                  style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}
                >
                  <Text>{getTargetName(target)}</Text>
                  {target.record.name && getTargetHostname(target) && (
                    <Text color="muted">{getTargetHostname(target)}</Text>
                  )}
                </View>
                {target.latestEvent && (
                  <Animated.Text
                    style={[
                      getStatusColor(target),
                      { fontSize: 11 },
                      target.latestEvent.status === "pending" && animatedStyle,
                    ]}
                  >
                    {target.latestEvent.status}
                  </Animated.Text>
                )}
              </View>
              <Switch
                value={target.record.active}
                onValueChange={(active) => toggleTarget(target, active)}
                disabled={togglingTargets.has(target.uri)}
              />
            </View>
          ))}
        </View>
      )}
    </View>
  );
}
