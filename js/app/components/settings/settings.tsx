import {
  selectTelemetry,
  setURL,
  telemetryOpt,
  selectStreamplace,
} from "features/streamplace/streamplaceSlice";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useState, useEffect } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, Form, H3, Input, View, XStack, Label, Text } from "tamagui";
import { Updates } from "./updates";
import { Switch } from "react-native";

const DEFAULT_PROD_URL = "https://stream.place";
const DEFAULT_DEV_URL = "http://10.0.1.5:38080";

export function Settings() {
  const dispatch = useAppDispatch();
  const { url } = useStreamplaceNode();
  const [newUrl, setNewUrl] = useState("");
  const [overrideEnabled, setOverrideEnabled] = useState(false);

  // Initialize the override state based on current URL
  useEffect(() => {
    setOverrideEnabled(url !== DEFAULT_PROD_URL);
  }, [url]);

  const onSubmit = () => {
    dispatch(setURL(newUrl));
    setNewUrl("");
  };

  const handleToggleOverride = (enabled: boolean) => {
    setOverrideEnabled(enabled);
    if (enabled) {
      dispatch(setURL(DEFAULT_DEV_URL));
    } else {
      dispatch(setURL(DEFAULT_PROD_URL));
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
        <Text fontSize="$4" color="$gray10" marginBottom="$3">
          Current node: {url}
        </Text>

        <XStack
          alignItems="center"
          justifyContent="space-around"
          width="100%"
          margin="$2"
        >
          <H3>Use custom node?</H3>
          <Switch
            style={{
              transform: [{ scaleX: 1.2 }, { scaleY: 1.2 }],
            }}
            value={overrideEnabled}
            onValueChange={handleToggleOverride}
          />
        </XStack>

        {overrideEnabled && (
          <XStack alignItems="center" gap="$2" marginTop="$2" width="100%">
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
              <Button size="$3">Go</Button>
            </Form.Trigger>
          </XStack>
        )}
      </Form>
      <View
        alignItems="center"
        justifyContent="center"
        gap="$2"
        fg={1}
        flexBasis={0}
      >
        <Label>
          <H3>Optional Player Telemetry</H3>
        </Label>
        <View backgroundColor="$outlineColor" padding="$5" borderRadius="$20">
          <Switch
            style={{
              transform: [{ scaleX: 1.5 }, { scaleY: 1.5 }],
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
        </View>
      </View>
    </View>
  );
}
