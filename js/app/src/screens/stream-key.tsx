import { useToastController } from "@tamagui/toast";
import Loading from "components/loading/loading";
import {
  clearStreamKeyRecord,
  createStreamKeyRecord,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import { useEffect, useState } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { View, Paragraph, Button } from "tamagui";

const Row = ({ children }: { children: React.ReactNode }) => {
  return (
    <View w="100%" f={1} fd="row" padding="$4">
      {children}
    </View>
  );
};

const Left = ({ children }: { children: React.ReactNode }) => {
  return (
    <View f={2} fb={0}>
      {children}
    </View>
  );
};

const Right = ({ children }: { children: React.ReactNode }) => {
  return (
    <View f={6} alignItems="stretch" fb={0}>
      {children}
    </View>
  );
};

export default function StreamKeyScreen() {
  const userProfile = useAppSelector(selectUserProfile);
  const url = useAppSelector((state) => state.aquareum.url);

  if (!userProfile) {
    return <Loading />;
  }
  return (
    <View f={1} ai="center" jc="center" gap="$4" w="100%" p="$4">
      <View w="100%" maxWidth={500}>
        <Row>
          <Left>
            <Paragraph>Service</Paragraph>
          </Left>
          <Right>
            <Paragraph>WHIP</Paragraph>
          </Right>
        </Row>
        <Row>
          <Left>
            <Paragraph>Server</Paragraph>
          </Left>
          <Right>
            <Paragraph>{url}</Paragraph>
          </Right>
        </Row>
        <Row>
          <Left>
            <Paragraph>Bearer Token</Paragraph>
          </Left>
          <Right>
            <StreamKey />
          </Right>
        </Row>
      </View>
    </View>
  );
}

export function StreamKey() {
  const dispatch = useAppDispatch();
  const [generating, setGenerating] = useState(false);
  const newKey = useAppSelector((state) => state.bluesky.newKey);
  const toast = useToastController();
  useEffect(() => {
    if (!newKey) {
      return;
    }
    (async () => {
      try {
        await navigator.clipboard.writeText(newKey.privateKey);
        toast.show("Copied!", {
          message: "Bearer token copied to clipboard",
        });
      } catch (e) {
        // not allowed. oh well.
      }
    })();
    return () => {
      dispatch(clearStreamKeyRecord());
    };
  }, [newKey]);
  if (generating) {
    return <Loading />;
  }
  if (newKey) {
    return <Paragraph fontFamily="$mono">{newKey.privateKey}</Paragraph>;
  }
  return (
    <Button
      onPress={async () => {
        try {
          setGenerating(true);
          await dispatch(createStreamKeyRecord({ store: false }));
        } catch (e) {
          console.error("failed to generate stream key", e);
        } finally {
          setGenerating(false);
        }
      }}
    >
      Generate Stream Key
    </Button>
  );
}
