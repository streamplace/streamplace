import { View } from "tamagui";

const maxContainerWidths = {
  sm: 300,
  md: 400,
  lg: 800,
  xl: 1200,
};

export default function Container({ children, ...props }) {
  return (
    <View f={1} justifyContent="flex-start" alignItems="center">
      <View
        width="100vw"
        maxWidth="30"
        p="$1"
        $sm={{
          maxWidth: maxContainerWidths.sm,
          mx: "auto",
          px: "$2",
        }}
        $md={{
          maxWidth: maxContainerWidths.md,
          mx: "auto",
          px: "$4",
        }}
        $lg={{
          width: maxContainerWidths.lg,
          mx: "auto",
          px: "$8",
        }}
        $gtLg={{
          maxWidth: maxContainerWidths.xl,
          mx: "auto",
          px: "$4",
        }}
        {...props}
      >
        {children}
      </View>
    </View>
  );
}
