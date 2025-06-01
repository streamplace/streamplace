import {
  DEFAULT_URL,
  selectTelemetry,
  setURL,
  telemetryOpt,
} from "features/streamplace/streamplaceSlice";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useEffect, useState } from "react";
import { Switch } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, Form, H3, Input, Text, View, XStack, isWeb } from "tamagui";
import { Updates } from "./updates";

export function Settings() {
  const dispatch = useAppDispatch();
  const { url } = useStreamplaceNode();
  const defaultUrl = DEFAULT_URL;
  const [newUrl, setNewUrl] = useState("");
  const [overrideEnabled, setOverrideEnabled] = useState(false);

  // Initialize the override state based on current URL
  useEffect(() => {
    setOverrideEnabled(url !== defaultUrl);
  }, [url, defaultUrl]);

  const onSubmit = () => {
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

  const telemetry = useAppSelector(selectTelemetry);

  return (
    <View f={1} alignItems="stretch" justifyContent="center" fg={1}>
      <Updates />
      <Form
        fg={1}
        flexBasis={0}
        alignItems="center"
        justifyContent="center"
        padding="$4"
        onSubmit={onSubmit}
      >
        <View
          alignItems="center"
          justifyContent="center"
          gap="$2"
          fg={1}
          flexBasis={0}
          backgroundColor="rgba(0, 0, 0, 0.1)"
        >
          <XStack alignItems="center" justifyContent="space-around">
            <View>
              <XStack width={isWeb ? "100%" : "75%"}>
                <H3 fontSize="$8">Use custom node</H3>
                <Switch
                  accessibilityLabel="Use custom node"
                  accessibilityHint="Toggle to use a custom node"
                  style={{
                    transform: [{ scaleX: 1.2 }, { scaleY: 1.2 }],
                    marginLeft: 20,
                    marginTop: isWeb ? 8 : 4,
                  }}
                  value={overrideEnabled}
                  onValueChange={handleToggleOverride}
                />
              </XStack>
              <Text
                fontSize="$6"
                color="$gray10"
                style={{ opacity: overrideEnabled ? 0 : 1 }}
                numberOfLines={1}
                ellipsizeMode="middle"
                maxWidth={280}
              >
                Default node: {url}
              </Text>
            </View>
          </XStack>

          <XStack
            alignItems="stretch"
            gap="$2"
            width={isWeb ? "100%" : "75%"}
            style={{
              opacity: overrideEnabled ? 1 : 0,
              marginTop: -15,
            }}
          >
            <Input
              value={newUrl}
              flex={1}
              size="$3"
              placeholder={url}
              onChangeText={(t) => setNewUrl(t)}
              onSubmitEditing={onSubmit}
              textContentType="URL"
              autoCapitalize="none"
              autoCorrect={false}
            />
            <Form.Trigger asChild>
              <Button size="$3">SAVE</Button>
            </Form.Trigger>
          </XStack>
        </View>
      </Form>
      <View
        alignItems="center"
        justifyContent="center"
        gap="$2"
        fg={1}
        flexBasis={0}
      >
        <XStack alignItems="center" gap="$6">
          <View>
            <H3 fontSize="$8">Player Telemetry</H3>
            <Text
              fontSize="$6"
              color="$gray10"
              style={{ position: "absolute", bottom: -15 }}
            >
              Optional
            </Text>
          </View>
          <Switch
            accessibilityLabel="Player Telemetry"
            accessibilityHint="Toggle to enable player telemetry"
            style={{
              transform: [{ scaleX: 1.2 }, { scaleY: 1.2 }],
              marginTop: isWeb ? 0 : 8,
            }}
            value={telemetry === true}
            onValueChange={(checked) => {
              if (checked === true) {
                dispatch(telemetryOpt(true));
              } else {
                dispatch(telemetryOpt(false));
              }
            }}
          />
        </XStack>
      </View>
    </View>
  );
}
