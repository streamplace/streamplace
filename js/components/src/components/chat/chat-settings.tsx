import { EllipsisVertical } from "lucide-react-native";
import { useEffect, useState } from "react";
import { Platform, Pressable, View } from "react-native";
import { Button, zero } from "../..";
import {
  ChatFilterCategory,
  useChatFilters,
  useSetChatFilters,
} from "../../streamplace-store";
import { useTheme } from "../../ui";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuGroup,
  DropdownMenuInfo,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
} from "../ui/dropdown";
import { Text } from "../ui/text";

export type { ChatFilterCategory };

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
  const storedFilters = useChatFilters();
  const setStoredFilters = useSetChatFilters();
  const [filters, setFilters] =
    useState<Set<ChatFilterCategory>>(storedFilters);

  const isMobile = Platform.OS === "ios" || Platform.OS === "android";

  // Sync local state with stored filters on mount and when stored filters change
  useEffect(() => {
    setFilters(storedFilters);
  }, [storedFilters]);

  const toggleFilter = (category: ChatFilterCategory) => {
    const newFilters = new Set(filters);
    if (newFilters.has(category)) {
      newFilters.delete(category);
    } else {
      newFilters.add(category);
    }
    setFilters(newFilters);
    setStoredFilters(newFilters);
    onFiltersChange?.(newFilters);
  };

  const allFiltersEnabled = filters.size === ALL_CATEGORIES.length;

  const toggleAllFilters = () => {
    const newFilters = allFiltersEnabled
      ? new Set<ChatFilterCategory>()
      : new Set(ALL_CATEGORIES);
    setFilters(newFilters);
    setStoredFilters(newFilters);
    onFiltersChange?.(newFilters);
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger>
        <Pressable>
          {({ pressed }) => (
            <Button
              variant="ghost"
              aria-label="Popout Chat"
              style={{ borderRadius: 16, maxHeight: 44, aspectRatio: 0.5 }}
            >
              <EllipsisVertical size={20} color={icons.color.muted} />
            </Button>
          )}
        </Pressable>
      </DropdownMenuTrigger>
      <ResponsiveDropdownMenuContent align="end">
        <DropdownMenuGroup title="Chat Settings">
          <DropdownMenuSub>
            <DropdownMenuSubTrigger subMenuTitle="Chat Filters">
              <View
                style={[
                  zero.flex.values[1],
                  isMobile ? zero.layout.flex.row : zero.layout.flex.column,
                  zero.layout.flex.spaceBetween,
                  zero.pr[4],
                ]}
              >
                <Text>Chat Filters</Text>
              </View>
            </DropdownMenuSubTrigger>
            <DropdownMenuSubContent>
              <DropdownMenuGroup title="Content Filters">
                <DropdownMenuItem onPress={toggleAllFilters}>
                  <Text>
                    {allFiltersEnabled ? "Disable All" : "Enable All"}
                  </Text>
                </DropdownMenuItem>
              </DropdownMenuGroup>
              <DropdownMenuGroup>
                {(
                  Object.entries(CATEGORY_LABELS) as [
                    ChatFilterCategory,
                    string,
                  ][]
                ).map(([category, label], i) => (
                  <>
                    <DropdownMenuCheckboxItem
                      key={category}
                      checked={filters.has(category)}
                      onCheckedChange={() => toggleFilter(category)}
                    >
                      <Text>{label}</Text>
                    </DropdownMenuCheckboxItem>
                    {i < Object.entries(CATEGORY_LABELS).length - 1 && (
                      <DropdownMenuSeparator />
                    )}
                  </>
                ))}
              </DropdownMenuGroup>
              <DropdownMenuInfo description="Hide messages containing content that may be inappropriate or offensive by category." />
            </DropdownMenuSubContent>
          </DropdownMenuSub>
        </DropdownMenuGroup>
      </ResponsiveDropdownMenuContent>
    </DropdownMenu>
  );
}
