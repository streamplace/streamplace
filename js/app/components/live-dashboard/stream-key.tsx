import { useToastController } from "@tamagui/toast";
import Loading from "components/loading/loading";
import {
  clearStreamKeyRecord,
  createStreamKeyRecord,
  selectUserProfile,
  selectIsReady,
} from "features/bluesky/blueskySlice";
import { useEffect, useState } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { View, Paragraph, Button, Text } from "tamagui";
import { Redirect } from "components/aqlink";
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
  const [protocol, setProtocol] = useState("whip");
  const isReady = useAppSelector(selectIsReady);
  if (!isReady) {
    return <Loading />;
  }
  const userProfile = useAppSelector(selectUserProfile);
  if (!userProfile) {
    return <Redirect to={{ screen: "Login" }} />;
  }
  const url = useAppSelector((state) => state.streamplace.url);

  if (!userProfile) {
    return <Loading />;
  }

  return (
    <View
      f={1}
      ai="center"
      jc="center"
      gap="$4"
      w="100%"
      p="$4"
      backgroundColor="$gray1"
    >
      <View w="100%" maxWidth={600}>
        <Row>
          <Button
            marginHorizontal={10}
            backgroundColor={
              protocol === "whip" ? "$accentBackground" : "$grey2"
            }
            onPress={() => setProtocol("whip")}
          >
            WHIP
          </Button>
          <Button
            marginHorizontal={10}
            backgroundColor={
              protocol === "rtmp" ? "$accentBackground" : "$grey2"
            }
            onPress={() => setProtocol("rtmp")}
          >
            RTMP (beta)
          </Button>
        </Row>
        {protocol === "whip" && <WHIPDescription url={url} />}
        {protocol === "rtmp" && <RTMPDescription url={url} />}
        <Row>
          <Left>
            <Paragraph>Output Settings</Paragraph>
          </Left>
          <Right>
            <Paragraph>Output mode: Advanced</Paragraph>
            <Paragraph>
              Keyframe Interval: <Text fontFamily="$mono">1s</Text>
            </Paragraph>
            <Paragraph>
              x264 Options: <Text fontFamily="$mono">bframes=0</Text>
            </Paragraph>
          </Right>
        </Row>
      </View>
    </View>
  );
}

export function WHIPDescription({ url }: { url: string }) {
  return (
    <>
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
    </>
  );
}

export function RTMPDescription({ url }: { url: string }) {
  const u = new URL(url);
  const rtmpUrl = `rtmps://${u.host}:1935/live`;
  return (
    <>
      <Row>
        <Left>
          <Paragraph>Service</Paragraph>
        </Left>
        <Right>
          <Paragraph>Custom...</Paragraph>
        </Right>
      </Row>
      <Row>
        <Left>
          <Paragraph>Server</Paragraph>
        </Left>
        <Right>
          <Paragraph>{rtmpUrl}</Paragraph>
        </Right>
      </Row>
      <Row>
        <Left>
          <Paragraph>Stream Key</Paragraph>
        </Left>
        <Right>
          <StreamKey />
        </Right>
      </Row>
    </>
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
