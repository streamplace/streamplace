import * as Popover from "@radix-ui/react-popover";
import { CircleHelp, X } from "lucide-react-native";
import { useId, useState } from "react";
import { View } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import * as zero from "../../ui";
import { IconButton } from "./icon-button";
import { Text } from "./text";
import type { WarningHelpProps } from "./warning-help";

export function WarningHelp({ warning }: WarningHelpProps) {
  const [open, setOpen] = useState(false);
  const descriptionId = useId();
  const { theme } = useTheme();
  const title = `About ${warning.label}`;

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <IconButton
          size="sm"
          accessibilityLabel={title}
          aria-describedby={open ? descriptionId : undefined}
          onPress={() => setOpen(!open)}
        >
          <CircleHelp size={16} color={theme.colors.textMuted} />
        </IconButton>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          asChild
          aria-label={title}
          aria-describedby={descriptionId}
          align="start"
          sideOffset={theme.spacing[1]}
          collisionPadding={theme.spacing[2]}
          onOpenAutoFocus={(event) => event.preventDefault()}
        >
          <View
            style={{
              ...zero.atomsNS.zIndex[50],
              ...zero.borders.width.thin,
              width: theme.spacing[72],
              maxWidth: "var(--radix-popover-content-available-width)" as any,
              padding: theme.spacing[3],
              borderRadius: theme.borderRadius.md,
              borderColor: theme.colors.borderStrong,
              backgroundColor: theme.colors.surface2,
              gap: theme.spacing[2],
              ...theme.shadows.lg,
            }}
          >
            <View
              style={{
                flexDirection: "row",
                alignItems: "center",
                justifyContent: "space-between",
                gap: theme.spacing[2],
              }}
            >
              <Text size="sm" weight="semibold" style={{ flexShrink: 1 }}>
                {warning.label}
              </Text>
              <Popover.Close asChild>
                <IconButton
                  size="sm"
                  accessibilityLabel="Close"
                  onPress={() => setOpen(false)}
                >
                  <X size={16} color={theme.colors.textMuted} />
                </IconButton>
              </Popover.Close>
            </View>
            <Text nativeID={descriptionId} size="sm">
              {warning.description}
            </Text>
          </View>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
