import { useNavigation } from "@react-navigation/native";
import { ArrowRight } from "@tamagui/lucide-icons";
import AQLink from "components/aqlink";
import Container from "components/container";
import {
  createServerSettingsRecord,
  getServerSettingsFromPDS,
  selectIsReady,
  selectServerSettings,
} from "features/bluesky/blueskySlice";
import { DEFAULT_URL, setURL } from "features/streamplace/streamplaceSlice";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useEffect, useState } from "react";
import { Switch } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, H3, H5, Input, Text, View, XStack } from "tamagui";
import { Updates } from "./updates";

export function Settings() {
  const dispatch = useAppDispatch();
  const { url } = useStreamplaceNode();
  const defaultUrl = DEFAULT_URL;
  const [newUrl, setNewUrl] = useState("");
  const [overrideEnabled, setOverrideEnabled] = useState(false);

  // are we logged in?
  const loggedIn = useAppSelector(
    (state) => state.bluesky.status === "loggedIn",
  );

  const navigate = useNavigation();

  // Initialize the override state based on current URL
  useEffect(() => {
    setOverrideEnabled(url !== defaultUrl);
  }, [url, defaultUrl]);

  const onSubmitUrl = () => {
    if (newUrl) {
      dispatch(setURL(newUrl));
      setNewUrl("");
    }
  };

  const handleToggleOverride = (enabled: boolean) => {
    setOverrideEnabled(enabled);
    if (!enabled) {
      dispatch(setURL(defaultUrl));
    }
  };

  return (
    <Container alignItems="center" justifyContent="center">
      <View
        f={1}
        alignItems="stretch"
        justifyContent="flex-start"
        mt="$8"
        maxWidth={500}
        $platform-web={{ width: "100%" }}
        gap="$6"
      >
        <View maxHeight={200}>
          <Updates />
        </View>

        <View alignItems="center" justifyContent="center" gap="$4">
          <XStack
            alignItems="stretch"
            justifyContent="space-between"
            width="100%"
            flexDirection="column"
          >
            <View
              flexDirection="row"
              alignItems="center"
              justifyContent="space-between"
              flex={1}
            >
              <View flex={1} pr="$3">
                <H3 fontSize="$7">Use Custom Node</H3>
                <Text fontSize="$5" color="$gray10">
                  Default: {url}
                </Text>
              </View>
              <Switch
                value={overrideEnabled}
                onValueChange={handleToggleOverride}
              />
            </View>
          </XStack>

          {/* Custom URL Input Row */}
          <XStack
            alignItems="center" // Changed to center
            gap="$2"
            style={{
              opacity: overrideEnabled ? 1 : 0,
              height: overrideEnabled ? "auto" : 0, // Collapse when hidden
              overflow: "hidden", // Hide overflow when collapsed
              transition: "opacity 0.2s ease-in-out, height 0.2s ease-in-out",
            }}
          >
            <Input
              value={newUrl}
              flex={1}
              size="$4"
              placeholder={url || "Enter custom node URL"}
              onChangeText={setNewUrl}
              onSubmitEditing={onSubmitUrl}
              textContentType="URL"
              autoCapitalize="none"
              autoCorrect={false}
              keyboardType="url"
            />
            <Button size="$4" onPress={onSubmitUrl}>
              <Text>SAVE</Text>
            </Button>
          </XStack>
        </View>

        {loggedIn && (
          <>
            <DebugRecording />
            <AQLink
              to={{
                screen: "KeyManagement",
              }}
            >
              <View
                flexDirection="row"
                gap="$2"
                alignItems="center"
                justifyContent="center"
                borderWidth={1}
                borderColor="$color.gray3Dark"
                padding="$2"
                borderRadius="$4"
                backgroundColor="$color.gray1Dark"
              >
                <H5>Manage Keys</H5>
                <ArrowRight size="$1" />
              </View>
            </AQLink>
          </>
        )}
      </View>
    </Container>
  );
}

const DebugRecording = () => {
  const dispatch = useAppDispatch();
  const isReady = useAppSelector(selectIsReady);
  const serverSettings = useAppSelector(selectServerSettings) || {};
  const { url } = useStreamplaceNode();

  useEffect(() => {
    if (isReady) {
      dispatch(getServerSettingsFromPDS());
    }
  }, [isReady]);

  const u = new URL(url);
  return (
    <View alignItems="center" justifyContent="center" gap="$4">
      <XStack alignItems="center" justifyContent="space-between" width="100%">
        <View flex={1} pr="$3">
          <H3 fontSize="$8">
            Allow {u.host} to record your livestream for debugging and improving
            the service
          </H3>
          <Text fontSize="$5" color="$gray10">
            Optional
          </Text>
        </View>
        <Switch
          value={serverSettings?.debugRecording === true}
          onValueChange={(value) => {
            if (value === true) {
              dispatch(
                createServerSettingsRecord({
                  ...serverSettings,
                  debugRecording: true,
                }),
              );
            } else {
              dispatch(
                createServerSettingsRecord({
                  ...serverSettings,
                  debugRecording: false,
                }),
              );
            }
          }}
        ></Switch>
      </XStack>
    </View>
  );
};
