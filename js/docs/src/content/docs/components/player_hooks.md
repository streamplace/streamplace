---
title: Player Hooks
description: A guide to the hooks available for building a custom player UI.
---

# Player Hooks

We’ve put together a set of hooks that give you the data and tools you need to
build great streaming experiences. You can mix and match these hooks to create
all kinds of different livestreaming experiences.

## Core Hooks

These are the primary hooks you'll use to get information about the livestream
and the player itself.

### `useLivestreamInfo`

This is the most important hook. It provides all the core data and actions for
the stream.

- **Returns:**
  - `ingest`: The current ingest status (various strings, or `null`).
    - `"new"`: Indicates that the stream is not live yet and can be started.
    - `null`: Indicates that the player is not being used to stream.
    - Other strings: Indicate different states of the stream.
  - `profile`: The profile information of the streamer.
  - `title`: The title of the stream.
  - `setTitle`: A function to update the stream title.
  - `showCountdown`: A boolean to control the visibility of the countdown
    overlay.
  - `setShowCountdown`: A function to toggle the countdown overlay.
  - `recordSubmitted`: A boolean that is `true` when the stream has just gone
    live.
  - `setRecordSubmitted`: A function to update the `recordSubmitted` state.
  - `ingestStarting`: A boolean that is `true` when the ingest server is
    starting up.
  - `setIngestStarting`: A function to update the `ingestStarting` state.
  - `connectionQuality`: The quality of the stream's connection.
  - `toggleGoLive`: A function to start or stop the livestream.

### `usePlayerDimensions`

This hook provides the dimensions of the video player.

- **Returns:**
  - `width`: The width of the player.
  - `height`: The height of the player.
  - `isPlayerRatioGreater`: A boolean that is `true` if the player's aspect
    ratio is greater than the video's aspect ratio.

### `useAvatars`

This hook fetches and returns avatar URLs for a given list of DIDs.

- **Arguments:**
  - An array of user DIDs (Decentralized Identifiers).
- **Returns:**
  - An object where the keys are DIDs and the values are avatar URLs.

## UI and Interaction Hooks

These hooks are used to manage UI state and interactions.

### `useKeyboardSlide`

This hook helps you adjust your UI when the keyboard is shown or hidden.

- **Returns:**
  - `slideKeyboard`: A value that can be used to animate the UI when the
    keyboard appears.

### `useCameraToggle`

This hook provides a function to switch between the front and back cameras.

- **Returns:**
  - `doSetIngestCamera`: A function to toggle the camera.

### `useSegmentTiming`

This hook provides metrics about the stream's performance.

- **Returns:**
  - `segmentDeltas`: An array of segment deltas.
  - `mean`: The mean segment delta.
  - `range`: The range of segment deltas.

## Example Usage

Here’s a simple example of how you might use these hooks in a custom player UI
component:

````tsx
import { useLivestreamInfo } from "@streamplace/components";
import { Pressable, Text, View } from "react-native";

export function MyMinimalUi() {
  const { ingest, title, toggleGoLive } = useLivestreamInfo();

  const isSelfAndNotLive = ingest === "new";

  return (
    <View style={{ flex: 1, justifyContent: "flex-end", padding: 20 }}>
      {isSelfAndNotLive ? (
        <Pressable
          onPress={toggleGoLive}
          style={{ backgroundColor: "red", padding: 15, borderRadius: 8 }}
        >
          <Text style={{ color: "white", textAlign: "center" }}>Go Live!</Text>
        </Pressable>
      ) : (
        <Text style={{ color: "white", fontSize: 24, fontWeight: "bold" }}>
          {title || "Welcome to the stream!"}
        </Text>
      )}
    </View>
  );
}```

You can read more about how to build custom UIs in the [Custom Player UI documentation](/docs/components/custom_ui).
````
