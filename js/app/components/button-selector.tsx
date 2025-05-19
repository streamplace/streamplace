import { YStack, XStack, Text, Button, View, YStackProps } from "tamagui";

export default function ButtonSelector({
  text,
  values,
  selectedValue,
  setSelectedValue,
  ...props
}: {
  text?: string;
  values: { label: string; value: string }[];
  selectedValue: string;
  setSelectedValue: (value: any) => void;
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
