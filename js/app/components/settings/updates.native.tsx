import { StatusBar } from "expo-status-bar";
import * as ExpoUpdates from "expo-updates";
import { useEffect, useState } from "react";
import { Button, H2, H5, ScrollView, Text, View } from "tamagui";
import Constants from "expo-constants";
import { ToastViewport, useToastController } from "@tamagui/toast";
import pkg from "../../package.json";
import { Platform } from "react-native";

export function Updates() {
  const version = pkg.version;
  const { currentlyRunning, isUpdateAvailable, isUpdatePending } =
    ExpoUpdates.useUpdates();

  const [checked, setChecked] = useState(false);

  useEffect(() => {
    if (isUpdatePending) {
      ExpoUpdates.reloadAsync();
    }
  }, [isUpdatePending]);

  useEffect(() => {
    if (isUpdateAvailable && checked) {
      ExpoUpdates.fetchUpdateAsync();
    }
  }, [isUpdateAvailable, checked]);

  // If true, we show the button to download and run the update
  const showDownloadButton = isUpdateAvailable;
  const buttonText = isUpdateAvailable
    ? "Download new update"
    : "Check for updates";

  // Show whether or not we are running embedded code or an update
  let runTypeMessage = currentlyRunning.isEmbeddedLaunch ? "Bundled" : "OTA";
  if (currentlyRunning.isEmergencyLaunch) {
    runTypeMessage = "Recovery";
  }

  const toast = useToastController();

  return (
    <View
      f={1}
      alignItems="center"
      justifyContent="center"
      fg={1}
      flexBasis={0}
    >
      <ToastViewport name="modal" top="$8" left={0} right={0} />
      <View>
        <H2 textAlign="center">Streamplace v{version}</H2>
        <H5 textAlign="center" pb="$5">
          {runTypeMessage}
        </H5>
        <Button
          onPress={async () => {
            try {
              setChecked(true);
              const res = await ExpoUpdates.checkForUpdateAsync();
              if (!res.isAvailable) {
                toast.show("No update found", {
                  viewportName: "modal",
                  message:
                    "You are on the latest version of Streamplace, hooray!",
                });
              }
            } catch (e) {
              toast.show("Update failed!", {
                viewportName: "modal",
                message: `You may need to update the app through the ${Platform.OS === "ios" ? "App" : "Play"} Store.`,
              });
            }
          }}
        >
          <Text>{buttonText}</Text>
        </Button>
      </View>
    </View>
  );
}
