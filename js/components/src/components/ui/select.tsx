import { Check, ChevronDown } from "lucide-react-native";
import { forwardRef } from "react";
import { View } from "react-native";
import { zero } from "../..";
import { useTheme } from "../../lib/theme/theme";
import { flex } from "../../ui";
import {
  DropdownMenu,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
} from "./dropdown";
import { Text } from "./text";

const { layout, px, py, borders, r, gap } = zero;

export interface SelectItem {
  label: string;
  value: string;
  description?: string;
}

export interface SelectProps {
  value?: string;
  onValueChange: (value: string) => void;
  placeholder?: string;
  items: SelectItem[];
  disabled?: boolean;
  style?: any;
}

export const Select = forwardRef<View, SelectProps>(
  (
    {
      value,
      onValueChange,
      placeholder = "Select...",
      items,
      disabled = false,
      style,
    },
    ref,
  ) => {
    const { theme } = useTheme();

    const selectedItem = items.find((item) => item.value === value);

    return (
      <DropdownMenu>
        <DropdownMenuTrigger disabled={disabled}>
          <View
            ref={ref}
            style={[
              {
                width: "100%",
                paddingHorizontal: theme.spacing[3],
                paddingVertical: theme.spacing[3],
                borderWidth: 1,
                borderColor: theme.colors.border,
                borderRadius: theme.borderRadius.md,
                backgroundColor: disabled
                  ? theme.colors.muted
                  : theme.colors.card,
                minHeight: theme.touchTargets.minimum,
                opacity: disabled ? 0.5 : 1,
              },
              style,
            ]}
          >
            <View
              style={{
                flexDirection: "row",
                alignItems: "center",
                justifyContent: "space-between",
                width: "100%",
                gap: 8,
              }}
            >
              <Text
                style={{
                  fontSize: 16,
                  color: disabled
                    ? theme.colors.textDisabled
                    : theme.colors.text,
                  flex: 1,
                }}
              >
                {selectedItem?.label || placeholder}
              </Text>
              <ChevronDown size={16} color={theme.colors.textMuted} />
            </View>
          </View>
        </DropdownMenuTrigger>

        <ResponsiveDropdownMenuContent
          align="start"
          style={[
            {
              maxHeight: 400,
            },
          ]}
        >
          {items.map((item, index) => (
            <View key={item.value}>
              <DropdownMenuItem onPress={() => onValueChange(item.value)}>
                <View
                  style={{
                    flexDirection: "row",
                    alignItems: "center",
                    justifyContent: "space-between",
                    width: "100%",
                    gap: 8,
                  }}
                >
                  <View style={[gap.all[1], py[1], flex.values[1]]}>
                    <Text
                      style={{
                        fontWeight: item.value === value ? "500" : "400",
                      }}
                      color={item.value === value ? "primary" : "default"}
                    >
                      {item.label}
                    </Text>
                    {item.description && (
                      <Text size="sm" color="muted">
                        {item.description}
                      </Text>
                    )}
                  </View>
                  {item.value === value ? (
                    <Check size={16} color={theme.colors.primary} />
                  ) : (
                    <View style={{ width: 16 }} />
                  )}
                </View>
              </DropdownMenuItem>
              {index < items.length - 1 && <DropdownMenuSeparator />}
            </View>
          ))}
        </ResponsiveDropdownMenuContent>
      </DropdownMenu>
    );
  },
);

Select.displayName = "Select";
