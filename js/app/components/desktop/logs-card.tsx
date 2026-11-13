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

interface AppBridge {
  openLogFolder(): Promise<void>;
}

type DesktopWindow = Window & { SP_ELECTRON?: { app?: AppBridge } };

export function LogsCard() {
  const { theme } = useTheme();
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
          <View style={{ flex: 1, gap: theme.spacing[3] }}>
            <Text size="base">The node's logs are saved on this computer.</Text>
            <Button testID="desktop-open-logs" onPress={openLogFolder}>
              Open log folder
            </Button>
          </View>
        </MenuItem>
      </MenuGroup>
    </MenuContainer>
  );
}
