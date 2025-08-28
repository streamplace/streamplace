import { Body, Button, Code, Row, View } from "@streamplace/components";
import { Redirect } from "components/aqlink";
import Loading from "components/loading/loading";
import {
  clearStreamKeyRecord,
  createStreamKeyRecord,
  selectIsReady,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import { useEffect, useState } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";

const FormRow = ({ children }: { children: React.ReactNode }) => {
  return (
    <Row fullWidth padding="lg" align="start">
      {children}
    </Row>
  );
};

const Label = ({ children }: { children: React.ReactNode }) => {
  return (
    <View flex={2}>
      <Body>{children}</Body>
    </View>
  );
};

const Content = ({ children }: { children: React.ReactNode }) => {
  return (
    <View flex={6} align="stretch">
      {children}
    </View>
  );
};

export function StreamKeyScreen() {
  const [protocol, setProtocol] = useState<"whip" | "rtmp">("rtmp");
  const isReady = useAppSelector(selectIsReady);

  if (!isReady) {
    return <Loading />;
  }

  const userProfile = useAppSelector(selectUserProfile);
  if (!userProfile) {
    return <Redirect to={{ screen: "Login" }} />;
  }

  const url = useAppSelector((state) => state.streamplace.url);

  return (
    <View flex={1} centered fullWidth padding="lg">
      <View fullWidth style={{ maxWidth: 600 }}>
        <FormRow>
          <Button
            variant={protocol !== "rtmp" ? "secondary" : "primary"}
            onPress={() => setProtocol("rtmp")}
          >
            RTMP
          </Button>
          <Button
            variant={protocol !== "whip" ? "secondary" : "primary"}
            onPress={() => setProtocol("whip")}
          >
            WHIP
          </Button>
        </FormRow>

        {protocol === "whip" && <WHIPDescription url={url} />}
        {protocol === "rtmp" && <RTMPDescription url={url} />}

        <FormRow>
          <Label>Output Settings</Label>
          <Content>
            <Body>Output mode: Advanced</Body>
            <Body>
              Keyframe Interval: <Code>1s</Code>
            </Body>
            <Body>
              x264 Options: <Code>bframes=0</Code>
            </Body>
          </Content>
        </FormRow>
      </View>
    </View>
  );
}

export function WHIPDescription({ url }: { url: string }) {
  return (
    <>
      <FormRow>
        <Label>Service</Label>
        <Content>
          <Body>WHIP</Body>
        </Content>
      </FormRow>
      <FormRow>
        <Label>Server</Label>
        <Content>
          <Body>{url}</Body>
        </Content>
      </FormRow>
      <FormRow>
        <Label>Bearer Token</Label>
        <Content>
          <StreamKey />
        </Content>
      </FormRow>
    </>
  );
}

export function RTMPDescription({ url }: { url: string }) {
  const u = new URL(url);
  const rtmpUrl = `rtmps://${u.host}:1935/live`;

  return (
    <>
      <FormRow>
        <Label>Service</Label>
        <Content>
          <Body>Custom...</Body>
        </Content>
      </FormRow>
      <FormRow>
        <Label>Server</Label>
        <Content>
          <Body>{rtmpUrl}</Body>
        </Content>
      </FormRow>
      <FormRow>
        <Label>Stream Key</Label>
        <Content>
          <StreamKey />
        </Content>
      </FormRow>
    </>
  );
}

export default StreamKeyScreen;

export function StreamKey() {
  const dispatch = useAppDispatch();
  const [generating, setGenerating] = useState(false);
  const newKey = useAppSelector((state) => state.bluesky.newKey);

  useEffect(() => {
    if (!newKey) {
      return;
    }

    (async () => {
      try {
        await navigator.clipboard.writeText(newKey.privateKey);
        // TODO: Replace with custom toast implementation
        console.log("Bearer token copied to clipboard");
      } catch (e) {
        // not allowed. oh well.
        console.log("Could not copy to clipboard");
      }
    })();

    return () => {
      dispatch(clearStreamKeyRecord());
    };
  }, [newKey, dispatch]);

  if (generating) {
    return <Loading />;
  }

  if (newKey) {
    return <Code>{newKey.privateKey}</Code>;
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
