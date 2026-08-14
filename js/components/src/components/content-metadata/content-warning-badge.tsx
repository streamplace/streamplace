import { AlertTriangle } from "lucide-react-native";
import { TextStyle, View } from "react-native";
import { zero } from "../..";
import { C2PA_WARNING_LABELS } from "../../lib/metadata-constants";
import { useTheme } from "../../lib/theme/theme";
import { hexToRgba, pt, r } from "../../ui";
import {
  DropdownMenu,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
} from "../ui/dropdown";
import { Text } from "../ui/text";

const { px, py, gap, layout } = zero;

export interface ContentWarningBadgeProps {
  warnings: string[];
  truncate?: boolean;
}

export function ContentWarningBadge({
  warnings,
  truncate = false,
}: ContentWarningBadgeProps) {
  const { theme } = useTheme();

  const getWarningLabel = (warning: string): string => {
    return C2PA_WARNING_LABELS[warning] || warning;
  };

  if (!warnings || warnings.length === 0) {
    return null;
  }

  const labelStyle: TextStyle = truncate
    ? {
        color: theme.colors.mutedForeground,
        flexShrink: 1,
        minWidth: 0,
        overflow: "hidden",
      }
    : { color: theme.colors.mutedForeground };

  return (
    <DropdownMenu style={truncate ? { flexShrink: 1, minWidth: 0 } : undefined}>
      <DropdownMenuTrigger>
        <View
          style={[
            layout.flex.row,
            layout.flex.align.center,
            gap.all[2],
            px[2],
            py[2],
            r.md,
            { backgroundColor: hexToRgba(theme.colors.muted, 0.53) },
          ]}
        >
          <AlertTriangle size={14} color={theme.colors.mutedForeground} />
          <Text
            size="sm"
            numberOfLines={truncate ? 1 : undefined}
            style={labelStyle}
          >
            Intended for certain audiences
          </Text>
        </View>
      </DropdownMenuTrigger>

      <ResponsiveDropdownMenuContent>
        <View style={[layout.flex.column, px[2], pt[2]]}>
          <Text>Heads up!</Text>
          <Text>This stream may contain:</Text>
        </View>
        <View
          style={[
            layout.flex.row,
            { flexWrap: "wrap" },
            gap.all[2],
            px[2],
            py[2],
          ]}
        >
          {warnings.map((warning, index) => (
            <View
              key={index}
              style={[
                { backgroundColor: theme.colors.warning },
                px[3],
                py[1],
                r.full,
              ]}
            >
              <Text
                size="sm"
                weight="semibold"
                style={{ color: theme.colors.warningForeground }}
              >
                {getWarningLabel(warning)}
              </Text>
            </View>
          ))}
        </View>
      </ResponsiveDropdownMenuContent>
    </DropdownMenu>
  );
}
