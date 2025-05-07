import { Text, View } from "tamagui";
import { useMedia } from "tamagui";

export default function BreakpointIndicator() {
  const media = useMedia();
  const breakpoints: (keyof typeof media)[] = ["xs", "sm", "md", "lg", "gtLg"];

  // Find the largest matching breakpoint
  let current = "default";
  for (const key of breakpoints) {
    if (media[key]) {
      console.log(key, media[key]);
      current = key;
    }
  }

  return (
    <View
      borderRadius={999}
      backgroundColor="$blue10Dark"
      height="$2"
      width="$2"
      alignItems="center"
      justifyContent="center"
    >
      <Text fontSize="$5" color="$color">
        {current}
      </Text>
    </View>
  );
}
