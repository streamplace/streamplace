import { Text, TextProps } from "tamagui";

export default function Title({
  children,
  ...props
}: TextProps & { children: React.ReactNode }) {
  return (
    <Text
      $sm={{
        fontSize: "$6",
      }}
      $md={{
        fontSize: "$7",
      }}
      $lg={{
        fontSize: "$8",
      }}
      $gtLg={{
        fontSize: "$9",
      }}
      fontSize="$9"
      fontWeight="bold"
      {...props}
    >
      {children}
    </Text>
  );
}
