import {
  selectTelemetry,
  setURL,
  telemetryOpt,
  DEFAULT_URL,
} from "features/streamplace/streamplaceSlice";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useState, useEffect } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, Form, H3, Input, View, XStack, Text, Switch } from "tamagui"; // Import Tamagui components
import { Updates } from "./updates";
import Container from "components/container";
import { useNavigation } from "@react-navigation/native";
import AQLink from "components/aqlink";

export function Settings() {
  const dispatch = useAppDispatch();
  const { url } = useStreamplaceNode();
  const defaultUrl = DEFAULT_URL;
  const [newUrl, setNewUrl] = useState("");
  const [overrideEnabled, setOverrideEnabled] = useState(false);

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

  const telemetry = useAppSelector(selectTelemetry);

  const handleTelemetryToggle = (checked: boolean) => {
    dispatch(telemetryOpt(checked));
  };

  return (
    <Container alignItems="center" justifyContent="center">
      <View
        f={1}
        alignItems="stretch"
        justifyContent="center"
        mt="$8"
        maxWidth={500}
        gap="$6"
        width="100%"
      >
        <Updates />

        <View alignItems="center" justifyContent="center" width="100%">
          {/* Toggle Switch for Custom Node */}
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
              <H3 fontSize="$7">Use custom node</H3>
              <Switch
                size="$3" // Tamagui Switch size
                checked={overrideEnabled}
                onCheckedChange={handleToggleOverride}
                theme="green" // Optional: use a theme color
              >
                <Switch.Thumb animation="bouncy" />
              </Switch>
            </View>
            {!overrideEnabled && (
              <Text
                fontSize="$5" // Slightly smaller text
                color="$gray10"
                style={{ opacity: overrideEnabled ? 0 : 1 }}
                numberOfLines={1}
                ellipsizeMode="middle"
              >
                Default: {url} {/* Shorter label */}
              </Text>
            )}
          </XStack>

          {/* Custom URL Input Row */}
          <XStack
            alignItems="center" // Changed to center
            gap="$2"
            width="100%" // Adjusted width
            style={{
              opacity: overrideEnabled ? 1 : 0,
              height: overrideEnabled ? "auto" : 0, // Collapse when hidden
              overflow: "hidden", // Hide overflow when collapsed
              transition: "opacity 0.2s ease-in-out, height 0.2s ease-in-out", // Add transition
            }}
          >
            <Input
              value={newUrl}
              flex={1}
              size="$4" // Slightly larger input
              placeholder={url || "Enter custom node URL"} // Fallback placeholder
              onChangeText={setNewUrl}
              onSubmitEditing={onSubmitUrl}
              textContentType="URL"
              autoCapitalize="none"
              autoCorrect={false}
              keyboardType="url" // Use URL keyboard
            />
            <Button size="$4" onPress={onSubmitUrl}>
              {" "}
              {/* Slightly larger button */}
              SAVE
            </Button>
          </XStack>
        </View>

        {/* Player Telemetry Section */}
        <View
          alignItems="center"
          justifyContent="center"
          gap="$4" // Increased gap
        >
          {/* Toggle Switch for Player Telemetry */}
          <XStack
            alignItems="center"
            justifyContent="space-between"
            width="100%"
          >
            {" "}
            {/* Adjusted width and justification */}
            <View flex={1} pr="$3">
              {" "}
              {/* Added padding right to text container */}
              <H3 fontSize="$7">Player Telemetry</H3>{" "}
              {/* Slightly smaller heading */}
              <Text
                fontSize="$5" // Slightly smaller text
                color="$gray10"
              >
                Optional
              </Text>
            </View>
            <Switch
              size="$3"
              checked={telemetry === true}
              onCheckedChange={handleTelemetryToggle}
              theme="purple"
            >
              <Switch.Thumb animation="bouncy" />
            </Switch>
          </XStack>
        </View>

        {/* Manage Keys Button */}
        <AQLink
          to={{
            screen: "Key Manager",
          }}
        >
          <Button
            size="$4" // Slightly larger button
            onPress={() => {
              // redirect to manage keys page
            }}
            theme="blue" // Optional: use a theme color
          >
            Manage Keys
          </Button>
        </AQLink>
      </View>
    </Container>
  );
}
