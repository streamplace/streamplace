import { View } from "tamagui";

const maxContainerWidths = {
  xxs: 440,
  xs: 440,
  sm: 440,
  md: 660,
  lg: 740,
  xl: 860,
  twoXl: 1260,
  threeXl: 1660,
};

export default function Container({ children, ...props }) {
  return (
    <View f={1} justifyContent="flex-start" alignItems="center">
      <View
        width="100vw"
        maxWidth={maxContainerWidths.xxs}
        p="$1"
        mx="auto"
        $xxs={{ maxWidth: maxContainerWidths.xs, px: "$2" }}
        $xs={{ maxWidth: maxContainerWidths.xs, px: "$2" }}
        $sm={{
          maxWidth: maxContainerWidths.sm,
          px: "$2",
        }}
        $md={{
          maxWidth: maxContainerWidths.md,
          px: "$4",
        }}
        $lg={{
          width: maxContainerWidths.lg,
          px: "$8",
        }}
        $xl={{
          maxWidth: maxContainerWidths.xl,
          px: "$4",
        }}
        $gtXl={{
          maxWidth: maxContainerWidths.twoXl,
          px: "$4",
        }}
        $gtXxl={{
          maxWidth: maxContainerWidths.threeXl,
          px: "$4",
        }}
        {...props}
      >
        {children}
      </View>
    </View>
  );
}
