import { Text, View, zero } from "@streamplace/components";
import { Linking, Pressable, ScrollView } from "react-native";

const Anchor = ({
  href,
  children,
  ...props
}: {
  href: string;
  children: React.ReactNode;
}) => (
  <Pressable onPress={() => Linking.openURL(href)} {...props}>
    <Text style={{ color: "#0066cc", textDecorationLine: "underline" }}>
      {children}
    </Text>
  </Pressable>
);

export default function AboutScreen() {
  return (
    <ScrollView>
      <View style={[{ maxWidth: 500, marginHorizontal: "auto" }]}>
        <Text variant="h4" size="2xl" style={[zero.mt[4]]}>
          What is Streamplace?
        </Text>
        <Text>
          Streamplace is the video layer for decentralized social networks.
          We're building open-source infrastructure around bringing high-quality
          video experiences to the AT Protocol, designed around user sovereignty
          and content authenticity.
        </Text>

        <Text variant="h4" size="xl">
          Open source single-binary node software
        </Text>
        <Text>
          Get up and running with one command. No complex configuration or deep
          video expertise required. Perfect for hackers and builders.
        </Text>

        <Text variant="h4" size="xl">
          User sovereignty by design
        </Text>
        <Text>
          All video content is cryptographically signed by creators and respects
          their consent preferences. Built on the same public key infrastructure
          as decentralized social networks.
        </Text>

        <Text variant="h4" size="xl">
          Familiar streaming experience
        </Text>
        <Text>
          Native apps for iOS, Android, and web that provide the rich video
          features users expect: livestreaming, clips, uploads, and more.
        </Text>

        <Text variant="h4" size="xl">
          Built for federation
        </Text>
        <Text>
          Seamlessly integrates with the AT Protocol. Streamplace nodes can
          connect to any compatible social network to index and serve video
          content.
        </Text>

        <Text variant="h4" size="xl">
          Powered by Livepeer
        </Text>
        <Text>
          Leverages battle-tested decentralized video infrastructure for
          transcoding, distribution, and delivery at scale.
        </Text>

        <Text variant="h4" size="xl">
          Want to get involved?
        </Text>
        <Text>
          Join our <Anchor href="https://discord.stream.place">Discord</Anchor>{" "}
          to learn more about Streamplace and how you can get involved.
        </Text>
      </View>
    </ScrollView>
  );
}
