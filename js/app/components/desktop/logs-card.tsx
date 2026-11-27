import {
  Button,
  MenuContainer,
  MenuGroup,
  MenuItem,
  MenuLabel,
  useToast,
} from "@streamplace/components";

interface AppBridge {
  openLogFolder(): Promise<void>;
}

type DesktopWindow = Window & { SP_ELECTRON?: { app?: AppBridge } };

export function LogsCard() {
  const toast = useToast();
  const bridge: AppBridge | undefined =
    typeof window === "undefined"
      ? undefined
      : (window as DesktopWindow).SP_ELECTRON?.app;
  if (!bridge) return null;

  const openLogFolder = async () => {
    try {
      await bridge.openLogFolder();
    } catch (error) {
      toast.show(
        "Logs",
        error instanceof Error
          ? error.message
          : "Couldn't open the log folder.",
        { variant: "error" },
      );
    }
  };

  return (
    <MenuContainer>
      <MenuLabel>Logs</MenuLabel>
      <MenuGroup>
        <MenuItem>
          <Button
            testID="desktop-open-logs"
            onPress={openLogFolder}
            width="min"
          >
            Open log folder
          </Button>
        </MenuItem>
      </MenuGroup>
    </MenuContainer>
  );
}
