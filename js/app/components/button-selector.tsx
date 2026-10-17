import { Text, useTheme, zero } from "@streamplace/components";
import { Pressable, View, ViewStyle } from "react-native";

interface ButtonSelectorProps {
  text?: string;
  values: { label: string; value: string }[];
  selectedValue: string;
  setSelectedValue: (value: any) => void;
  style?: ViewStyle;
}

export default function ButtonSelector({
  text,
  values,
  selectedValue,
  setSelectedValue,
  style,
  ...props
}: ButtonSelectorProps) {
  const { theme } = useTheme();
  return (
    <View
      style={[{ alignItems: "flex-start" }, zero.gap.all[2], zero.pt[2], style]}
      {...props}
    >
      {text && (
        <Text style={[{ fontSize: 16 }, { fontWeight: "600" }]}>{text}</Text>
      )}
      <View
        style={[
          zero.layout.flex.row,
          zero.layout.flex.alignCenter,
          zero.layout.flex.spaceAround,
          zero.gap.all[1],
          zero.w.percent[100],
          { backgroundColor: theme.colors.surface2 },
          zero.r.xl,
        ]}
      >
        {values.map(({ label, value }) => (
          <Pressable
            key={value}
            onPress={() => setSelectedValue(value)}
            style={[
              zero.flex.values[1],
              zero.h[32], // height equivalent to "$2"
              selectedValue === value
                ? [
                    zero.borders.width.medium,
                    { borderColor: theme.colors.border },
                    zero.r.lg,
                  ]
                : [],
              zero.layout.flex.center,
            ]}
          >
            <Text
              style={[
                {
                  color:
                    selectedValue === value
                      ? theme.colors.text1
                      : theme.colors.text3,
                },
              ]}
            >
              {label}
            </Text>
          </Pressable>
        ))}
      </View>
    </View>
  );
}
