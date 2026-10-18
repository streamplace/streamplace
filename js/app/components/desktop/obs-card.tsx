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
import { useStore } from "store";

interface ObsStatus {
  obs: "absent" | "installed" | "flatpak";
  plugin: "absent" | "outdated" | "current";
  obsRunning: boolean;
  bundledVersion: string;
  installedVersion?: string;
}

interface Result {
  ok: boolean;
  error?: string;
}

// What Streamplace Desktop's preload exposes (js/desktop/src/preload.ts).
interface ObsBridge {
  status(): Promise<ObsStatus>;
  installPlugin(): Promise<Result & { restartObs: boolean }>;
  configureStream(streamKey: string): Promise<Result>;
}

const obsBridge = (): ObsBridge | undefined =>
  typeof window === "undefined" ? undefined : (window as any).SP_ELECTRON?.obs;

const describe = (s: ObsStatus): string => {
  if (s.obs === "absent") return "OBS isn't installed.";
  if (s.obs === "flatpak")
    return "Flatpak OBS can't load the Streamplace plugin; install OBS from your distribution or obsproject.com.";
  if (s.plugin === "absent") return "The Streamplace plugin isn't installed.";
  if (s.plugin === "outdated")
    return `The Streamplace plugin is out of date (${s.installedVersion}).`;
  return `The Streamplace plugin is installed (${s.bundledVersion}).`;
};

/**
 * Streamplace Desktop's OBS setup: installing the plugin it ships, and
 * pointing OBS at this node with a new stream key. Nothing outside Desktop.
 */
export function ObsCard() {
  const { theme } = useTheme();
  const toast = useToast();
  const bridge = obsBridge();
  const [status, setStatus] = useState<ObsStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const createStreamKeyRecord = useStore((s) => s.createStreamKeyRecord);
  const clearStreamKeyRecord = useStore((s) => s.clearStreamKeyRecord);

  const refresh = useCallback(async () => {
    if (bridge) setStatus(await bridge.status());
  }, [bridge]);
  useEffect(() => {
    refresh();
  }, [refresh]);

  if (!bridge || !status) {
    return null;
  }

  const run = async (action: () => Promise<Result>, done: string) => {
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
    } finally {
      setBusy(false);
    }
  };

  const install = () =>
    run(
      () => bridge.installPlugin(),
      status.obsRunning
        ? "Plugin installed. Restart OBS to load it."
        : "Plugin installed.",
    );

  // A fresh key each time: Desktop never sees a key's secret again after
  // creating it, so connecting OBS is what creates one.
  const connect = () =>
    run(async () => {
      await createStreamKeyRecord(false);
      const key = useStore.getState().newKey;
      clearStreamKeyRecord();
      if (!key) {
        return { ok: false, error: "Couldn't create a stream key." };
      }
      return bridge.configureStream(key.privateKey);
    }, "OBS now streams to Streamplace Desktop.");

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
              <Button onPress={install} disabled={busy}>
                {status.plugin === "absent"
                  ? "Install the plugin"
                  : "Update the plugin"}
              </Button>
            )}
            {installable && status.plugin === "current" && (
              <>
                <Button onPress={connect} disabled={busy || status.obsRunning}>
                  Connect OBS to Desktop
                </Button>
                {status.obsRunning && (
                  <Text size="sm" color="muted">
                    Close OBS to change its stream settings.
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
