import { Loader, Text, View, zero } from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { useCallback, useEffect, useState } from "react";
import { Switch } from "react-native";
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

  if (loading && targets.length === 0) {
    return (
      <View style={[p[3]]}>
        <Text style={[text.gray[400], { fontSize: 14 }]}>
          Loading multistream...
        </Text>
      </View>
    );
  }

  if (targets.length === 0) {
    return (
      <View style={[p[3]]}>
        <Text style={[text.gray[400], { fontSize: 14 }]}>
          No multistream targets configured
        </Text>
      </View>
    );
  }

  return (
    <View style={[p[3], gap.all[2]]}>
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
        <Text size="xl" style={[text.white]}>
          Multistream {loading && <Loader size="small" />}
        </Text>
      </View>

      {targets.map((target) => (
        <View
          key={target.uri}
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            layout.flex.spaceBetween,
            p[2],
            bg.neutral[800],
            r.md,
            borders.width.thin,
            borders.color.neutral[700],
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
  );
}
