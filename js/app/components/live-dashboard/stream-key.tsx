import { useRoute } from "@react-navigation/native";
import {
  Body,
  Button,
  Code,
  Row,
  Text,
  useStreamplaceStore,
  useTheme,
  useToast,
  View,
  zero,
} from "@streamplace/components";
import {
  fontFamilies,
  typeScale,
} from "@streamplace/components/src/lib/theme/tokens";
import useGetIngests from "@streamplace/components/src/streamplace-store/ingest";
import Loading from "components/loading/loading";
import { Clipboard, ClipboardCheck } from "lucide-react-native";
import { useEffect, useState } from "react";
import { ScrollView, TextInput } from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";
import { PlaceStreamIngestDefs } from "streamplace";

const FormRow = ({ children }: { children: React.ReactNode }) => {
  return (
    <Row fullWidth align="start">
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
  const [ingest, setIngest] = useState<PlaceStreamIngestDefs.Ingest | null>(
    null,
  );
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const route = useRoute();
  const url = useStore((state) => state.url);
  const ingests = useStreamplaceStore((state) => state.ingests);
  const getIngests = useGetIngests();
  useEffect(() => {
    getIngests();
  }, []);

  useEffect(() => {
    if (isReady && !userProfile) {
      openLoginModal({ name: route.name, params: route.params });
    }
  }, [isReady, userProfile, openLoginModal, route.name, route.params]);

  useEffect(() => {
    if (ingests && ingests.length > 0 && !ingest) {
      setIngest(ingests[0]);
    }
  }, [ingests, ingest]);

  if (!isReady) {
    return <Loading />;
  }

  if (!userProfile) {
    return <Loading />;
  }

  if (!ingests) {
    return <Loading />;
  }

  return (
    <ScrollView>
      <View flex={1} align="center" justify="start" padding="md" fullWidth>
        <View fullWidth style={{ maxWidth: 600 }}>
          <FormRow>
            {ingests.map((ing, i) => (
              <Button
                width="min"
                key={i}
                variant={ingest !== ing ? "secondary" : "primary"}
                onPress={() => setIngest(ing)}
              >
                {ing.type.toUpperCase()}
              </Button>
            ))}
          </FormRow>
          {ingest?.type === "whip" && <WHIPDescription url={ingest.url} />}
          {ingest?.type.startsWith("rtmp") && (
            <RTMPDescription url={ingest.url} />
          )}
          <FormRow>
            <Label>Output Settings</Label>
            <Content>
              <View style={[zero.mt[2]]}>
                <Text>Output mode: Advanced</Text>
                <Text>
                  Keyframe Interval: <Code>1s</Code>
                </Text>
                <Text>
                  x264 Options:{" "}
                  <Code
                    style={{
                      paddingHorizontal: 4,
                    }}
                  >
                    bframes=0
                  </Code>
                </Text>
                <Text
                  underline
                  style={{
                    fontWeight: "bold",
                  }}
                >
                  (Very important!)
                </Text>
              </View>
            </Content>
          </FormRow>
        </View>
      </View>
    </ScrollView>
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
          <TextInput
            value={url}
            readOnly={true}
            style={[
              {
                backgroundColor: zero.tokens.surfaces.dark[1],
                borderWidth: 1,
                borderColor: zero.tokens.borderAlphas.dark.default,
                borderRadius: zero.borderRadius.md,
                padding: zero.spacing[3],
                color: zero.tokens.textAlphas.dark[1],
                fontFamily: fontFamilies.monoRegular,
                fontSize: typeScale.base.fontSize,
              },
            ]}
          />
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
          <TextInput
            value={url}
            readOnly={true}
            style={[
              {
                backgroundColor: zero.tokens.surfaces.dark[1],
                borderWidth: 1,
                borderColor: zero.tokens.borderAlphas.dark.default,
                borderRadius: zero.borderRadius.md,
                padding: zero.spacing[3],
                color: zero.tokens.textAlphas.dark[1],
                fontFamily: fontFamilies.monoRegular,
                fontSize: typeScale.base.fontSize,
              },
            ]}
          />
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
  const theme = useTheme();
  const toast = useToast();

  const createStreamKeyRecord = useStore(
    (state) => state.createStreamKeyRecord,
  );
  const clearStreamKeyRecord = useStore((state) => state.clearStreamKeyRecord);
  const [generating, setGenerating] = useState(false);
  const [hidekey, setHidekey] = useState(true);
  const [didcopy, setDidcopy] = useState(false);
  const newKey = useStore((state) => state.newKey);

  let foregroundColor = theme.theme.colors.text1;

  useEffect(() => {
    if (!newKey) {
      return;
    }

    return () => {
      clearStreamKeyRecord();
    };
  }, [newKey]);

  const handleCopy = async () => {
    if (!newKey) {
      return;
    }

    try {
      await navigator.clipboard.writeText(newKey.privateKey);
      setDidcopy(true);

      toast.show("Stream Key", "Stream Key was copied to your clipboard", {
        duration: 4,
      });
    } catch (e) {
      // not allowed. oh well.
      toast.show(
        "Stream Key",
        "Failed to copy the Stream Key to your clipboard",
        { duration: 4 },
      );
    }
  };

  if (generating) {
    return <Loading />;
  }

  if (newKey) {
    return (
      <Row fullWidth flex={1} align="start">
        <TextInput
          value={newKey.privateKey}
          secureTextEntry={hidekey}
          readOnly={true}
          style={[
            {
              backgroundColor: zero.tokens.surfaces.dark[1],
              borderWidth: 1,
              borderColor: zero.tokens.borderAlphas.dark.default,
              borderRadius: zero.borderRadius.md,
              padding: zero.spacing[3],
              color: zero.tokens.textAlphas.dark[1],
              fontFamily: fontFamilies.monoRegular,
              fontSize: typeScale.base.fontSize,
              flex: 1,
              borderTopRightRadius: "0px",
              borderBottomRightRadius: "0px",
            },
          ]}
          onFocus={(e) => {
            setHidekey(false);
          }}
          onBlur={() => {
            setHidekey(true);
          }}
          selectTextOnFocus={true}
        />
        <Button
          width="min"
          onPress={handleCopy}
          style={[
            {
              borderTopLeftRadius: "0px",
              borderBottomLeftRadius: "0px",
            },
          ]}
        >
          {didcopy ? (
            <ClipboardCheck color={foregroundColor} size={24} />
          ) : (
            <Clipboard color={foregroundColor} size={24} />
          )}
        </Button>
      </Row>
    );
  }

  return (
    <Button
      onPress={async () => {
        try {
          setGenerating(true);
          setDidcopy(false);
          await createStreamKeyRecord(false);
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
