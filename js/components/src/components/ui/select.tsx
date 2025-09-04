import { ChevronDown } from "lucide-react-native";
import { forwardRef, useState } from "react";
import {
  FlatList,
  Modal,
  StyleSheet,
  TouchableOpacity,
  View,
} from "react-native";
import { useTheme } from "../../lib/theme/theme";
import { Text } from "./text";

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

export const Select = forwardRef<any, SelectProps>(
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
    const [isOpen, setIsOpen] = useState(false);

    const selectedItem = items.find((item) => item.value === value);

    const handleSelect = (itemValue: string) => {
      onValueChange(itemValue);
      setIsOpen(false);
    };

    const styles = createStyles(theme, disabled);

    return (
      <>
        <TouchableOpacity
          ref={ref}
          style={[styles.container, style]}
          onPress={() => !disabled && setIsOpen(true)}
          disabled={disabled}
        >
          <Text style={styles.value}>{selectedItem?.label || placeholder}</Text>
          <ChevronDown size={16} color={theme.colors.textMuted} />
        </TouchableOpacity>

        <Modal
          visible={isOpen}
          transparent
          animationType="fade"
          onRequestClose={() => setIsOpen(false)}
        >
          <TouchableOpacity
            style={styles.overlay}
            activeOpacity={1}
            onPress={() => setIsOpen(false)}
          >
            <View style={styles.dropdown}>
              <FlatList
                data={items}
                keyExtractor={(item) => item.value}
                renderItem={({ item }) => (
                  <TouchableOpacity
                    style={[
                      styles.item,
                      item.value === value && styles.selectedItem,
                    ]}
                    onPress={() => handleSelect(item.value)}
                  >
                    <Text
                      style={[
                        styles.itemText,
                        item.value === value ? styles.selectedItemText : {},
                      ]}
                    >
                      {item.label}
                    </Text>
                    {item.description && (
                      <Text style={styles.itemDescription}>
                        {item.description}
                      </Text>
                    )}
                  </TouchableOpacity>
                )}
                style={styles.list}
              />
            </View>
          </TouchableOpacity>
        </Modal>
      </>
    );
  },
);

Select.displayName = "Select";

function createStyles(theme: any, disabled: boolean) {
  return StyleSheet.create({
    container: {
      flexDirection: "row",
      alignItems: "center",
      justifyContent: "space-between",
      paddingHorizontal: theme.spacing[3],
      paddingVertical: theme.spacing[3],
      borderWidth: 1,
      borderColor: theme.colors.border,
      borderRadius: theme.borderRadius.md,
      backgroundColor: disabled ? theme.colors.muted : theme.colors.card,
      minHeight: theme.touchTargets.minimum,
    },
    value: {
      fontSize: 16,
      color: disabled ? theme.colors.textDisabled : theme.colors.text,
      flex: 1,
    },
    overlay: {
      flex: 1,
      backgroundColor: "rgba(0, 0, 0, 0.5)",
      justifyContent: "center",
      alignItems: "center",
    },
    dropdown: {
      backgroundColor: theme.colors.background,
      borderRadius: theme.borderRadius.md,
      borderWidth: 1,
      borderColor: theme.colors.border,
      maxHeight: 300,
      width: "90%",
      maxWidth: 400,
      ...theme.shadows.lg,
    },
    list: {
      maxHeight: 300,
    },
    item: {
      paddingHorizontal: theme.spacing[4],
      paddingVertical: theme.spacing[3],
      borderBottomWidth: 1,
      borderBottomColor: theme.colors.border,
    },
    selectedItem: {
      backgroundColor: theme.colors.primary,
    },
    itemText: {
      fontSize: 16,
      color: theme.colors.text,
    },
    selectedItemText: {
      color: theme.colors.primaryForeground,
      fontWeight: "500",
    },
    itemDescription: {
      fontSize: 14,
      color: theme.colors.textMuted,
      marginTop: theme.spacing[1],
    },
  });
}
