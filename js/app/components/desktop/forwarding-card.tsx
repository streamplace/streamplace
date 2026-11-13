import {
  MenuContainer,
  MenuGroup,
  MenuItem,
  MenuLabel,
  Text,
  useTheme,
  View,
} from "@streamplace/components";
import { useEffect, useRef, useState } from "react";
import { Animated, Easing, Platform } from "react-native";
import { useOAuthSession, useStreamplaceUrl } from "store/hooks";

interface PushPeer {
  peer: string;
  moqURL: string;
  connected: boolean;
  connectedAt: string;
  lastError?: string;
}

interface PushStatus {
  peers: PushPeer[];
  live: string[];
}

function FlowLink({ active }: { active: boolean }) {
  const { theme } = useTheme();
  const progress = useRef(new Animated.Value(0)).current;
  useEffect(() => {
    if (!active) {
      progress.setValue(0);
      return;
    }
    const animation = Animated.loop(
      Animated.timing(progress, {
        toValue: 1,
        duration: theme.motion.slow * 4,
        easing: Easing.linear,
        useNativeDriver: Platform.OS !== "web",
      }),
    );
    animation.start();
    return () => animation.stop();
  }, [active, progress, theme.motion.slow]);

  return (
    <View
      accessibilityElementsHidden
      style={{
        width: theme.spacing[8],
        height: theme.spacing[3],
        justifyContent: "center",
      }}
    >
      <View
        style={{
          height: theme.spacing[1],
          backgroundColor: theme.colors.borderSubtle,
          borderRadius: theme.borderRadius.full,
        }}
      />
      {active && (
        <Animated.View
          style={{
            position: "absolute",
            width: theme.spacing[2],
            height: theme.spacing[2],
            borderRadius: theme.borderRadius.full,
            backgroundColor: theme.colors.live,
            transform: [
              {
                translateX: progress.interpolate({
                  inputRange: [0, 1],
                  outputRange: [0, theme.spacing[8] - theme.spacing[2]],
                }),
              },
            ],
          }}
        />
      )}
    </View>
  );
}

export function ForwardingCard() {
  const { theme } = useTheme();
  const nodeURL = useStreamplaceUrl();
  const session = useOAuthSession();
  const [status, setStatus] = useState<PushStatus | null>(null);
  const [fetchError, setFetchError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const refresh = async () => {
      try {
        const response = await fetch(`${nodeURL}/api/push`, {
          signal: controller.signal,
        });
        if (response.status === 404) {
          setStatus(null);
          return;
        }
        if (!response.ok) {
          throw new Error(
            `Could not read forwarding status (${response.status}).`,
          );
        }
        const next: PushStatus = await response.json();
        if (stopped) return;
        setStatus(next);
        setFetchError(null);
      } catch (error) {
        if (stopped) return;
        setFetchError(
          error instanceof Error ? error.message : "Could not reach this node.",
        );
      }
      if (!stopped) timer = setTimeout(refresh, 2000);
    };
    refresh();
    return () => {
      stopped = true;
      clearTimeout(timer);
      controller.abort();
    };
  }, [nodeURL]);

  if (!status || status.peers.length === 0) return null;
  const live = !!session?.did && status.live.includes(session.did);

  return (
    <MenuContainer>
      <MenuLabel>Stream forwarding</MenuLabel>
      <MenuGroup>
        {status.peers.map((peer) => {
          const destination = peer.moqURL || peer.peer.split("?")[0];
          const connected = peer.connected && !fetchError;
          const forwarding = connected && live;
          const state = forwarding
            ? `Forwarding to ${destination}`
            : connected
              ? `Connected to ${destination} — ready and waiting for a stream.`
              : `Connecting to ${destination}`;
          const error = fetchError || (!connected && peer.lastError);
          return (
            <MenuItem key={peer.peer}>
              <View style={{ flex: 1, gap: theme.spacing[3] }}>
                <View
                  style={{
                    flexDirection: "row",
                    alignItems: "center",
                    gap: theme.spacing[2],
                  }}
                >
                  <Text size="sm">OBS</Text>
                  <FlowLink active={forwarding} />
                  <Text size="sm">This computer</Text>
                  <FlowLink active={forwarding} />
                  <Text
                    size="sm"
                    testID="desktop-forwarding-url"
                    style={{ flex: 1, flexShrink: 1 }}
                  >
                    {destination}
                  </Text>
                </View>
                <Text
                  size="base"
                  testID="desktop-forwarding-state"
                  accessibilityLiveRegion="polite"
                >
                  {state}
                </Text>
                {!!error && (
                  <Text size="sm" style={{ color: theme.colors.warning }}>
                    {error}
                  </Text>
                )}
              </View>
            </MenuItem>
          );
        })}
      </MenuGroup>
    </MenuContainer>
  );
}
