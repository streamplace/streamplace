import { Settings } from "lucide-react-native";
import { useState } from "react";
import { Pressable, View } from "react-native";
import { a, layout, p } from "../../lib/theme/atoms";
import { useTheme } from "../../ui";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuGroup,
  DropdownMenuInfo,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
} from "../ui/dropdown";
import { Text } from "../ui/text";

export type ChatFilterCategory =
  | "place.stream.richtext.defs#discriminatory"
  | "place.stream.richtext.defs#sexually_explicit"
  | "place.stream.richtext.defs#profanity";

interface ChatSettingsProps {
  onFiltersChange?: (filters: Set<ChatFilterCategory>) => void;
}

const CATEGORY_LABELS: Record<ChatFilterCategory, string> = {
  "place.stream.richtext.defs#discriminatory": "Discriminatory",
  "place.stream.richtext.defs#sexually_explicit": "Sexually Explicit",
  "place.stream.richtext.defs#profanity": "Profanity",
};

const ALL_CATEGORIES: ChatFilterCategory[] = [
  "place.stream.richtext.defs#discriminatory",
  "place.stream.richtext.defs#sexually_explicit",
  "place.stream.richtext.defs#profanity",
];

export function ChatSettings({ onFiltersChange }: ChatSettingsProps) {
  const { icons } = useTheme();
  const [filters, setFilters] = useState<Set<ChatFilterCategory>>(new Set());

  const toggleFilter = (category: ChatFilterCategory) => {
    const newFilters = new Set(filters);
    if (newFilters.has(category)) {
      newFilters.delete(category);
    } else {
      newFilters.add(category);
    }
    setFilters(newFilters);
    onFiltersChange?.(newFilters);
  };

  const allFiltersEnabled = filters.size === ALL_CATEGORIES.length;

  const toggleAllFilters = () => {
    const newFilters = allFiltersEnabled
      ? new Set<ChatFilterCategory>()
      : new Set(ALL_CATEGORIES);
    setFilters(newFilters);
    onFiltersChange?.(newFilters);
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger>
        <Pressable>
          {({ pressed }) => (
            <View
              style={[
                p[2],
                a.radius.all.sm,
                pressed && a.opacity[70],
                layout.flex.row,
                layout.flex.alignCenter,
              ]}
            >
              <Settings size={20} color={icons.color.muted} />
            </View>
          )}
        </Pressable>
      </DropdownMenuTrigger>
      <ResponsiveDropdownMenuContent align="end">
        <DropdownMenuLabel>Chat Settings</DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuGroup title="Chat Filters">
          <DropdownMenuItem onPress={toggleAllFilters}>
            <Text>{allFiltersEnabled ? "Disable All" : "Enable All"}</Text>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          {(
            Object.entries(CATEGORY_LABELS) as [ChatFilterCategory, string][]
          ).map(([category, label]) => (
            <DropdownMenuCheckboxItem
              key={category}
              checked={filters.has(category)}
              onCheckedChange={() => toggleFilter(category)}
            >
              <Text>{label}</Text>
            </DropdownMenuCheckboxItem>
          ))}
          <DropdownMenuInfo description="Hide messages containing content that may be inappropriate or offensive by category." />
        </DropdownMenuGroup>
      </ResponsiveDropdownMenuContent>
    </DropdownMenu>
  );
}
