---
title: Creating your own player UI
description:
  How to set up your player UI with components from @streamplace/components.
---

# Building a Custom Player UI

This guide will walk you through the process of creating your own custom player
UI for Streamplace. We'll use the existing `MobileUi` component as a reference
to understand how to leverage the available hooks and components.

## Core Concepts

The mobile UI is built using React Native and relies on a set of custom hooks to
manage state and interactions related to the livestream. By composing these
hooks and the UI components from `@streamplace/components`, you can build a
unique viewing and streaming experience.

- **React Hooks:** State management and logic are encapsulated in hooks (e.g.,
  `useLivestreamInfo`, `usePlayerDimensions`). This keeps the UI component clean
  and focused on rendering.
- **Component Library:** A shared component and UI library
  (`@streamplace/components`) provides ready-to-use UI elements like `PlayerUI`,
  `Toast`, and `View`.
- **Styling:** A theme-based styling system (`atoms`) is used for consistency.

## Reference Implementation

Let's break down the default `MobileUi` component to understand its structure.

```tsx
// streamplace/js/app/components/mobile/ui.tsx

import { useNavigation } from "@react-navigation/native";
import {
  atoms,
  PlayerUI,
  Text,
  Toast,
  useAvatars,
  useCameraToggle,
  useKeyboardSlide,
  useLivestreamInfo,
  usePlayerDimensions,
  useSegmentTiming as useSegmentMetrics,
  View,
} from "@streamplace/components";
import { ChevronLeft, SwitchCamera } from "lucide-react-native";
import { useEffect } from "react";
import { Image, Pressable } from "react-native";
// Ready-to-use chat component is not available yet
import { ChatPanel } from "./chat";

const { borders, colors, gap, h, layout, position, w } = atoms;

export function MobileUi() {
  // 1. Hooks for State and Logic
  const navigation = useNavigation();
  const {
    ingest,
    profile,
    title,
    setTitle,
    showCountdown,
    setShowCountdown,
    recordSubmitted,
    setRecordSubmitted,
    ingestStarting,
    setIngestStarting,
    connectionQuality,
    toggleGoLive,
  } = useLivestreamInfo();
  const { width, height, isPlayerRatioGreater } = usePlayerDimensions();
  const { slideKeyboard } = useKeyboardSlide();
  const { segmentDeltas, mean, range } = useSegmentMetrics();
  const { doSetIngestCamera } = useCameraToggle();
  const avatars = useAvatars(profile?.did ? [profile?.did] : []);

  // 2. Effects and Derived State
  useEffect(() => {
    return () => {
      if (ingestStarting) {
        setIngestStarting(false);
      }
    };
  }, [ingestStarting, setIngestStarting]);

  const isSelfAndNotLive = ingest === "new";
  const isLive = ingest !== null && ingest !== "new";

  // 3. JSX for Rendering
  return (
    <>
      {/* Top Bar: Navigation, Profile, and Camera Switch */}
      <View style={[layout.position.absolute, h.percent[100], w.percent[100]]}>
        {/* ... */}
      </View>

      {/* Metrics Panel */}
      {isLive && (
        <View>
          <PlayerUI.MetricsPanel
            connectionQuality={connectionQuality}
            showMetrics={isLive || isSelfAndNotLive}
            segmentDeltas={segmentDeltas}
            mean={mean || 999}
            range={range || 999}
          />
        </View>
      )}

      {/* Input Panel (for streamer) or Chat Panel (for viewer, or streamer when live) */}
      {isSelfAndNotLive ? (
        <PlayerUI.InputPanel
          title={title}
          setTitle={setTitle}
          ingestStarting={ingestStarting}
          toggleGoLive={toggleGoLive}
          slideKeyboard={slideKeyboard}
        />
      ) : (
        <ChatPanel
          isPlayerRatioGreater={isPlayerRatioGreater}
          slideKeyboard={slideKeyboard}
        />
      )}

      {/* Overlays and Toasts */}
      <PlayerUI.CountdownOverlay
        visible={showCountdown}
        width={width}
        height={height}
        startFrom={3}
        onDone={() => setShowCountdown(false)}
      />
      <Toast
        open={recordSubmitted}
        onOpenChange={setRecordSubmitted}
        title="You're live!"
        description="We're notifying your followers that you just went live."
        duration={5}
      />
    </>
  );
}
```

## Creating Your Own UI

### 1. Set Up the File

Create a new file for your custom UI component, for example, `MyCustomUi.tsx`,
inside `streamplace/js/app/components/mobile/`.

### 2. Import Necessary Hooks and Components

Start by importing the tools you'll need. The more important ones are:

- `useLivestreamInfo`: Provides all core data and actions for the stream (e.g.,
  `ingest` status, `profile` info, `toggleGoLive` function).
- `usePlayerDimensions`: Gives you the `width` and `height` of the video player
  area.
- `@streamplace/components`: Our source for pre-built hooks, UI elements
  (`PlayerUI`, `View`, `Text`, etc.) and styling `atoms`.
- `react-native`: For basic components like `Pressable` and `Image`.

:::note Feel free to copy our components into your own application if you want
to use a different UI library or framework. :::

### 3. Fetch State with Hooks

Inside your component function, call the hooks to get the data you need to drive
your UI.

```tsx
import {
  useLivestreamInfo,
  usePlayerDimensions,
} from "@streamplace/components";

export function MyCustomUi() {
  const { ingest, profile, title, setTitle, toggleGoLive } =
    useLivestreamInfo();
  const { width, height } = usePlayerDimensions();

  const isSelfAndNotLive = ingest === "new";
  const isLive = ingest !== null && ingest !== "new";

  // ... rest of your component
}
```

### 4. Structure Your Layout

Use the `View` component and the styling `atoms` to build your layout. You can
position elements absolutely to overlay them on top of the video stream.

The `isLive` and `isSelfAndNotLive` booleans are very useful for conditionally
rendering different parts of your UI.

- When `isSelfAndNotLive` is `true`, one is setting up their own stream but
  hasn't started it yet. This is a good time to show controls for setting a
  title and a "Go Live" button.
- When `isLive` is `true`, the stream is active. You can show UI that would be
  importing if one is streaming, such as a chat panel or metrics.
- If both are `false`, one is likely watching someone else's stream, thus it
  would be a good idea to show a chat panel or other viewer-focused controls.

### Example: A Minimal Custom UI

Here is a simple example that only shows a "Go Live" button for the streamer and
a title for everyone else.

```tsx
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
}
```

By following this structure, you can replace the default `MobileUi` with your
own implementation while reusing the underlying application logic provided by
the custom hooks.
