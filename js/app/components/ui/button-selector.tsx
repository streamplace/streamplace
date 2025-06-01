import { Button, Text, XStack, YStack, YStackProps } from "tamagui";

export default function ButtonSelector({
  text,
  values,
  selectedValue,
  setSelectedValue,
  disabledValues,
  ...props
}: {
  text?: string;
  values: { label: string; value: string }[];
  selectedValue: string;
  setSelectedValue: (value: any) => void;
  disabledValues?: string[];
} & YStackProps) {
  return (
    <YStack ai="flex-start" gap="$2" pt="$2" {...props}>
      {text && (
        <Text fontSize="$base" fontWeight="semibold">
          {text}
        </Text>
      )}
      <XStack
        ai="center"
        jc="space-around"
        gap="$1"
        w="100%"
        bg="$background"
        borderRadius="$xl"
      >
        {values.map(({ label, value }) => (
          <Button
            key={value}
            onPress={() => setSelectedValue(value)}
            f={1}
            height="$2"
            disabled={disabledValues?.includes(value)}
            opacity={disabledValues?.includes(value) ? 0.5 : 1}
            variant={selectedValue === value ? "outlined" : undefined}
          >
            <Text
              color={
                selectedValue === value
                  ? "$color.foreground"
                  : "$color.mutedForeground"
              }
            >
              {label}
            </Text>
          </Button>
        ))}
      </XStack>
    </YStack>
  );
}
