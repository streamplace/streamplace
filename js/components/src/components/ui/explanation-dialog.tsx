import { CircleHelp } from "lucide-react-native";
import { useRef, useState } from "react";
import { View } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import { Button } from "./button";
import { Dialog } from "./dialog";
import { IconButton } from "./icon-button";
import { Text } from "./text";

export function ExplanationDialog({
  title,
  entries,
  compact = false,
}: {
  title: string;
  entries: { label: string; description: string }[];
  compact?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const trigger = useRef<any>(null);
  const { theme } = useTheme();
  const close = () => {
    setOpen(false);
    requestAnimationFrame(() => trigger.current?.focus());
  };

  return (
    <>
      {compact ? (
        <IconButton
          ref={trigger}
          size="sm"
          accessibilityLabel={title}
          onPress={() => setOpen(true)}
        >
          <CircleHelp size={16} color={theme.colors.textMuted} />
        </IconButton>
      ) : (
        <Button
          ref={trigger}
          variant="ghost"
          size="sm"
          width="min"
          onPress={() => setOpen(true)}
        >
          {title}
        </Button>
      )}
      <Dialog
        title={title}
        size={compact ? "sm" : "md"}
        open={open}
        onOpenChange={(next) => (next ? setOpen(true) : close())}
      >
        <View style={{ gap: theme.spacing[4] }}>
          {entries.map((entry) => (
            <View key={entry.label} style={{ gap: theme.spacing[1] }}>
              {!compact && <Text weight="semibold">{entry.label}</Text>}
              <Text size="sm">{entry.description}</Text>
            </View>
          ))}
        </View>
      </Dialog>
    </>
  );
}
