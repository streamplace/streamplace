import { Text, useTheme } from "@streamplace/components";
import { ThumbnailSelectorProps } from "./shared";

export default function ThumbnailSelector(props: ThumbnailSelectorProps) {
  const { theme } = useTheme();
  return <Text style={[{ color: theme.colors.text1 }]}>NYI</Text>;
}
