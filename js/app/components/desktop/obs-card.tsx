import {
  Button,
  MenuContainer,
  MenuGroup,
  MenuItem,
  MenuLabel,
  Text,
  useTheme,
  useToast,
  View,
} from "@streamplace/components";
import { useCallback, useEffect, useState } from "react";

interface ObsStatus {
  obs: "absent" | "installed" | "flatpak";
  plugin: "absent" | "outdated" | "current";
  obsRunning: boolean;
  bundledVersion: string;
  installedVersion?: string;
  connected: boolean;
}

interface Result {
  ok: boolean;
  error?: string;
}

// What Streamplace Desktop's preload exposes (js/desktop/src/preload.ts).
interface ObsBridge {
  status(): Promise<ObsStatus>;
  installPlugin(): Promise<Result & { restartObs: boolean }>;
  configureStream(): Promise<Result>;
}

const obsBridge = (): ObsBridge | undefined =>
  typeof window === "undefined" ? undefined : (window as any).SP_ELECTRON?.obs;

const describe = (s: ObsStatus): string => {
  if (s.obs === "absent") return "OBS isn't installed";
  if (s.obs === "flatpak")
    return "Flatpak OBS can't load the Streamplace plugin";
  if (s.plugin === "absent") return "Streamplace plugin not installed";
  if (s.plugin === "outdated") return "Streamplace plugin out of date";
  return "Streamplace plugin installed";
};

// Streamplace Desktop's bundled plugin and local OBS profile setup.
export function ObsCard() {
  const { theme } = useTheme();
  const toast = useToast();
  const bridge = obsBridge();
  const [status, setStatus] = useState<ObsStatus | null>(null);
  const [busy, setBusy] = useState(false);

  const refresh = useCallback(async () => {
    if (bridge) setStatus(await bridge.status());
  }, [bridge]);
  useEffect(() => {
    void refresh();
    // Also notice a plugin installed by Desktop's startup prompt.
    const timer = setInterval(refresh, 3000);
    return () => clearInterval(timer);
  }, [refresh]);

  const run = useCallback(
    async (action: () => Promise<Result>, done: string) => {
      setBusy(true);
      try {
        const result = await action();
        if (result.ok) {
          toast.show("OBS", done);
        } else {
          toast.show("OBS", result.error ?? "Something went wrong.", {
            variant: "error",
          });
        }
        await refresh();
      } catch (error) {
        toast.show("OBS", String(error), { variant: "error" });
      } finally {
        setBusy(false);
      }
    },
    [toast, refresh],
  );

  const connect = () => {
    if (bridge) void run(() => bridge.configureStream(), "OBS connected.");
  };

  if (!bridge || !status) return null;

  const install = () =>
    run(
      () => bridge.installPlugin(),
      status.obsRunning
        ? "Plugin installed. Restart OBS to load it."
        : "Plugin installed.",
    );

  const installable = status.obs === "installed";
  return (
    <MenuContainer>
      <MenuLabel>OBS</MenuLabel>
      <MenuGroup>
        <MenuItem>
          <View style={{ flex: 1, gap: theme.spacing[3] }}>
            <Text size="base" testID="desktop-obs-status">
              {describe(status)}
            </Text>
            {installable && status.plugin !== "current" && (
              <Button onPress={install} disabled={busy} width="min">
                {status.plugin === "absent"
                  ? "Install the plugin"
                  : "Update the plugin"}
              </Button>
            )}
            {installable && status.plugin === "current" && (
              <>
                {status.connected && (
                  <Text size="sm" testID="desktop-obs-connected">
                    OBS connected
                  </Text>
                )}
                <Button
                  onPress={connect}
                  disabled={busy || status.obsRunning}
                  width="min"
                >
                  Connect OBS
                </Button>
                {status.obsRunning && (
                  <Text size="sm" color="muted">
                    Close OBS to connect it.
                  </Text>
                )}
              </>
            )}
          </View>
        </MenuItem>
      </MenuGroup>
    </MenuContainer>
  );
}
