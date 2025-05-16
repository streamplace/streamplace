import { View } from "tamagui";

const maxContainerWidths = {
  xxs: 440,
  xs: 440,
  sm: 440,
  md: 660,
  lg: 740,
  xl: 800,
  twoXl: 1260,
  threeXl: 1660,
};

export default function Container({ children, ...props }) {
  return (
    <View f={1} justifyContent="flex-start" alignItems="center">
      <View
        width="100vw"
        px="$4"
        mx="auto"
        $gtXxs={{ maxWidth: maxContainerWidths.sm, px: "$4" }}
        $gtXs={{
          maxWidth: maxContainerWidths.lg,
          px: "$4",
        }}
        $gtLg={{
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
