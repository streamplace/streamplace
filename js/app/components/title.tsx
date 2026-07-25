import { Text } from "@streamplace/components";
import { TextStyle } from "react-native";

interface TitleProps {
  children: React.ReactNode;
  style?: TextStyle;
}

export default function Title({ children, style, ...props }: TitleProps) {
  return (
    <Text size="2xl" style={[style as any]} {...props}>
      {children}
    </Text>
  );
}
