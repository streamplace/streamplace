import { LivestreamProvider } from "@streamplace/components";
import { Camera, FerrisWheel, X } from "@tamagui/lucide-icons";
import { Redirect } from "components/aqlink";
import CreateLivestream from "components/create-livestream";
import UpdateLivestream from "components/edit-livestream";
import StreamKeyScreen from "components/live-dashboard/stream-key";
import Waiting from "components/live-dashboard/waiting";
import Loading from "components/loading/loading";
import { Player } from "components/player/player";
import Popup from "components/popup";
import ButtonSelector from "components/ui/button-selector";
import { VideoElementProvider } from "contexts/VideoElementContext";
import {
  createServerSettingsRecord,
  getServerSettingsFromPDS,
  selectIsReady,
  selectServerSettings,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import { useLiveUser } from "hooks/useLiveUser";
import React, { useCallback, useEffect, useState } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, H3, H6, isWeb, Text, View } from "tamagui";

enum StreamSource {
  Start,
  Camera,
  StreamKey,
}

export default function LiveDashboard() {
  const isReady = useAppSelector(selectIsReady);
  const userProfile = useAppSelector(selectUserProfile);
  const [streamSource, setStreamSource] = useState(StreamSource.Start);
  const serverSettings = useAppSelector(selectServerSettings);
  const isLive = useLiveUser();
  const [videoElement, setVideoElement] = useState<HTMLVideoElement | null>(
    null,
  );
  const [gotSettings, setGotSettings] = useState(false);
  const dispatch = useAppDispatch();
  useEffect(() => {
    if (isReady) {
      (async () => {
        await dispatch(getServerSettingsFromPDS());
        setGotSettings(true);
      })();
    }
  }, [isReady]);

  let madeChoiceAboutDebugRecording = true;
  if (gotSettings && serverSettings?.debugRecording === undefined) {
    madeChoiceAboutDebugRecording = false;
  }

  const [page, setPage] = useState<"update" | "create">("create");

  const videoRef = useCallback((node: HTMLVideoElement | null) => {
    if (node !== null) {
      setVideoElement(node);
    }
  }, []);
  if (!isReady) {
    return <Loading />;
  }
  if (!userProfile) {
    return <Redirect to={{ screen: "Login" }} />;
  }
  let topPane: React.ReactNode;
  let params = new URLSearchParams();
  if (isWeb) {
    params = new URLSearchParams(window.location.search);
  }

  if (isLive && streamSource !== StreamSource.Camera) {
    topPane = (
      <Player
        src={userProfile.did}
        name={userProfile.handle}
        videoRef={videoRef}
      />
    );
  } else if (streamSource === StreamSource.Start) {
    topPane = <StreamSourcePicker onPick={setStreamSource} />;
  } else if (streamSource === StreamSource.Camera) {
    topPane = (
      <Player src={userProfile.did} name={userProfile.handle} ingest={true} />
    );
  } else if (streamSource === StreamSource.StreamKey) {
    topPane = <StreamKeyScreen />;
  } else {
    throw new Error("Invalid stream source");
  }
  let closeButton: React.ReactNode = <></>;
  if (streamSource !== StreamSource.Start && !isLive) {
    closeButton = (
      <Button
        position="absolute"
        top="$0"
        right="$0"
        onPress={(e) => {
          e.stopPropagation();
          setStreamSource(StreamSource.Start);
        }}
        zIndex={1000}
        marginTop={10}
        marginRight={10}
      >
        <X />
      </Button>
    );
  }

  return (
    <LivestreamProvider src={userProfile.did}>
      <VideoElementProvider videoElement={videoElement}>
        <View f={1} ai="stretch" jc="center">
          <View f={1} fb={0}>
            {topPane}
            {closeButton}
          </View>
          <View f={1} ai="center" jc="center" fb={0}>
            <ButtonSelector
              values={[
                { label: "Create", value: "create" },
                { label: "Update", value: "update" },
              ]}
              disabledValues={isLive ? [] : ["update"]}
              selectedValue={page}
              setSelectedValue={setPage}
              maxWidth={250}
              width="100%"
            />
            {page === "update" && isLive ? <UpdateLivestream /> : null}
            {page === "create" ? <CreateLivestream /> : null}
          </View>
          {madeChoiceAboutDebugRecording ? null : <DebugRecordingPopup />}
        </View>
      </VideoElementProvider>
    </LivestreamProvider>
  );
}

const elems = [
  {
    title: "Stream your camera!",
    Icon: Camera,
    to: StreamSource.Camera,
  },
  {
    title: "Stream from OBS!",
    Icon: FerrisWheel,
    to: StreamSource.StreamKey,
  },
];

export function DebugRecordingPopup() {
  const dispatch = useAppDispatch();
  const serverSettings = useAppSelector(selectServerSettings) || {};
  const opt = (choice) => () =>
    dispatch(
      createServerSettingsRecord({
        ...serverSettings,
        debugRecording: choice,
      }),
    );
  return (
    <Popup
      onClose={opt(false)}
      containerProps={{
        bottom: "$8",
        zIndex: 1000,
      }}
      bubbleProps={{
        backgroundColor: "$accentBackground",
        gap: "$3",
        maxWidth: 400,
      }}
    >
      <H3 textAlign="center">Debug Recording</H3>
      <Text>
        Streamplace is beta software and it helps us to archive livestreams so
        we can later use them for debugging. Would you like to opt in to debug
        recording?
      </Text>
      <View flexDirection="row" gap="$2" f={1}>
        <Button f={3} backgroundColor="$accentColor" onPress={opt(true)}>
          Allow
        </Button>
        <Button f={3} onPress={opt(false)}>
          Don't Allow
        </Button>
      </View>
    </Popup>
  );
}

export function StreamSourcePicker({
  onPick,
}: {
  onPick: (source: StreamSource) => void;
}) {
  const isReady = useAppSelector(selectIsReady);
  const userProfile = useAppSelector(selectUserProfile);
  if (!isReady) {
    return <Loading />;
  }
  if (!userProfile) {
    return <Redirect to={{ screen: "Login" }} />;
  }
  return (
    <View
      f={1}
      jc="space-around"
      ai="stretch"
      padding="$3"
      flexDirection="row"
      backgroundColor="$gray1"
    >
      <View f={1} maxWidth={250} alignItems="stretch" justifyContent="center">
        {elems.map(({ Icon, title, to }, i) => (
          <React.Fragment key={i}>
            <View
              f={1}
              flexDirection="row"
              ai="center"
              jc="space-between"
              backgroundColor="$accentColor"
              // padding="$5"
              borderRadius="$10"
              cursor="pointer"
              onPress={() => onPick(to)}
              flexGrow={0}
              flexBasis={75}
            >
              <View padding="$5" paddingRight={0}>
                <Icon size={48} />
              </View>
              <Text f={1} textAlign="right" paddingRight="$5">
                {title}
              </Text>
            </View>
            {i < elems.length - 1 && (
              <View jc="center" ai="center">
                <H6 padding="$5">OR</H6>
              </View>
            )}
          </React.Fragment>
        ))}
        <Waiting />
      </View>
    </View>
  );
}
