import { useNavigation } from "@react-navigation/native";
import { Button, Input, Text, View, zero } from "@streamplace/components";
import AQLink from "components/aqlink";
import {
  createServerSettingsRecord,
  getServerSettingsFromPDS,
  selectIsReady,
  selectServerSettings,
} from "features/bluesky/blueskySlice";
import { DEFAULT_URL, setURL } from "features/streamplace/streamplaceSlice";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useEffect, useState } from "react";
import { ScrollView, Switch } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import MultistreamManager from "./multistream-manager";
import { Updates } from "./updates";
import WebhookManager from "./webhook-manager";

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
      let trimmedUrl = newUrl.endsWith("/") ? newUrl.slice(0, -1) : newUrl;
      dispatch(setURL(trimmedUrl));
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
    <ScrollView>
      <View style={[{ alignItems: "center" }]}>
        <View
          style={[
            { gap: 32 },
            { paddingVertical: 24, maxWidth: 500, width: "100%" },
          ]}
        >
          <View>
            <Updates />
          </View>

          <View
            style={[
              { alignItems: "stretch" },
              { justifyContent: "flex-start" },
              { gap: 16 },
            ]}
          >
            <View
              style={[
                { alignItems: "stretch" },
                { justifyContent: "flex-start" },
                { width: "100%", flexDirection: "column" },
              ]}
            >
              <View
                style={[
                  { flexDirection: "row" },
                  { alignItems: "flex-start" },
                  { justifyContent: "flex-start" },
                ]}
              >
                <View style={[{ flex: 1 }, { paddingRight: 12 }]}>
                  <Text size="xl">Use Custom Node</Text>
                  <Text size="lg" color="muted">
                    Default: {url}
                  </Text>
                </View>
                <Switch
                  value={overrideEnabled}
                  onValueChange={handleToggleOverride}
                />
              </View>
            </View>

            {/* Custom URL Input Row */}
            <View
              style={[
                {
                  opacity: overrideEnabled ? 1 : 0,
                  height: overrideEnabled ? "auto" : 0,
                },
                zero.gap.all[2],
                zero.layout.flex.align.center,
                zero.layout.flex.row,
              ]}
            >
              <Input
                value={newUrl}
                containerStyle={[
                  { flex: 1, flexGrow: 1, width: "100%" },
                  zero.flex.grow[1],
                ]}
                variant="default"
                numberOfLines={1}
                multiline={false}
                placeholder={url || "Enter custom node URL"}
                placeholderTextColor="#999"
                onChangeText={setNewUrl}
                onSubmitEditing={onSubmitUrl}
                textContentType="URL"
                autoCapitalize="none"
                autoCorrect={false}
                keyboardType="url"
              />
              <Button
                size="md"
                variant="secondary"
                style={[zero.py[0], zero.flex.shrink[1]]}
                onPress={onSubmitUrl}
              >
                <Text size="lg">Save</Text>
              </Button>
            </View>
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
                  style={[
                    {
                      flexDirection: "row",
                      gap: 8,
                      alignItems: "center",
                      justifyContent: "center",
                      borderWidth: 1,
                      borderColor: "#333",
                      padding: 8,
                      borderRadius: 16,
                      backgroundColor: "#1a1a1a",
                    },
                  ]}
                >
                  <Text>Manage Keys</Text>
                  <Text style={[{ fontSize: 16 }]}>→</Text>
                </View>
              </AQLink>
              <WebhookManager />
              <MultistreamManager />
            </>
          )}
        </View>
      </View>
    </ScrollView>
  );
}

const DebugRecording = () => {
  const dispatch = useAppDispatch();
  const isReady = useAppSelector(selectIsReady);
  const serverSettings = useAppSelector(selectServerSettings);
  const { url } = useStreamplaceNode();
  const debugRecordingOn = serverSettings?.debugRecording === true;

  useEffect(() => {
    if (isReady) {
      dispatch(getServerSettingsFromPDS());
    }
  }, [isReady]);

  const u = new URL(url);
  return (
    <View
      style={[
        { alignItems: "center" },
        { justifyContent: "center" },
        { gap: 16 },
      ]}
    >
      <View
        style={[
          { alignItems: "center" },
          { justifyContent: "space-between" },
          { width: "100%", flexDirection: "row" },
        ]}
      >
        <View style={[{ flex: 1 }, { paddingRight: 12 }]}>
          <Text size="xl">
            Allow {u.host} to record your livestream for debugging and improving
            the service
          </Text>
          <Text size="lg" color="muted">
            Optional
          </Text>
        </View>
        <Switch
          value={debugRecordingOn}
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
        />
      </View>
    </View>
  );
};
