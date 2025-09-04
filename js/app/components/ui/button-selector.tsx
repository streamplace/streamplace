import {
  Button,
  Text,
  useTheme,
  View,
  ViewProps,
  zero,
} from "@streamplace/components";

const { gap, pt, w, bg, r, spacing, layout, colors } = zero;

export default function ButtonSelector({
  text,
  values,
  selectedValue,
  setSelectedValue,
  disabledValues,
  style,
  ...props
}: {
  text?: string;
  values: { label: string; value: string }[];
  selectedValue: string;
  setSelectedValue: (value: any) => void;
  disabledValues?: string[];
} & ViewProps) {
  let theme = useTheme();
  return (
    <View align="start" style={[gap.all[2], style as any]} {...props}>
      {text && (
        <Text variant="body1" weight="semibold">
          {text}
        </Text>
      )}
      <View
        direction="row"
        align="center"
        justify="around"
        style={[gap.all[1], w.percent[100], r.full]}
      >
        {values.map(({ label, value }) => {
          const isSelected = selectedValue === value;
          const isDisabled = disabledValues?.includes(value);

          return (
            <Button
              key={value}
              onPress={() => setSelectedValue(value)}
              variant={isSelected ? "outline" : "ghost"}
              size="pill"
              disabled={isDisabled}
              style={[
                { flex: 1, maxHeight: 20 },
                isSelected
                  ? { backgroundColor: theme.theme.colors.primary }
                  : { backgroundColor: theme.theme.colors.secondary },
                isDisabled && { opacity: 0.5 },
              ]}
            >
              {label}
            </Button>
          );
        })}
      </View>
    </View>
  );
}
