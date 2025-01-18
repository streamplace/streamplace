import { H4 as TamaguiH4, Paragraph, View, Anchor } from "tamagui";

const H4 = (props: any) => <TamaguiH4 fontWeight="100" {...props} />;
const P = (props: any) => (
  <Paragraph fontSize={13} marginBottom={20} {...props} />
);

export default function AboutScreen() {
  return (
    <View maxWidth={500} marginHorizontal="auto">
      <H4>What is Streamplace?</H4>
      <P fontSize={13} marginBottom={20}>
        Streamplace is the video layer for decentralized social networks. We're
        building open-source infrastructure to bring high-quality video
        experiences to the AT Protocol ecosystem, while preserving user
        sovereignty and content authenticity.
      </P>

      <H4>Open source single-binary node software</H4>
      <P>
        Get up and running with one command. No complex configuration or deep
        video expertise required. Perfect for hackers and builders.
      </P>

      <H4>User sovereignty by design</H4>
      <P>
        All video content is cryptographically signed by creators and respects
        their consent preferences. Built on the same public key infrastructure
        as decentralized social networks.
      </P>

      <H4>Familiar streaming experience</H4>
      <P>
        Native apps for iOS, Android, and web that provide the rich video
        features users expect: livestreaming, clips, uploads, and more.
      </P>

      <H4>Built for federation</H4>
      <P>
        Seamlessly integrates with the AT Protocol. Streamplace nodes can
        connect to any compatible social network to index and serve video
        content.
      </P>

      <H4>Powered by Livepeer</H4>
      <P>
        Leverages battle-tested decentralized video infrastructure for
        transcoding, distribution, and delivery at scale.
      </P>

      <H4>Want to get involved?</H4>
      <P>
        Join our <Anchor href="https://di/livepeer">Discord</Anchor> to learn
        more about Streamplace and how you can get involved.
      </P>
    </View>
  );
}
