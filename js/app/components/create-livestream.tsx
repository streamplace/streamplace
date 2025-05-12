import { useEffect, useState } from "react";
import { Button, Label, Paragraph, TextArea, View, isWeb } from "tamagui";
import { useToastController } from "@tamagui/toast";
import {
  createLivestreamRecord,
  selectNewLivestream,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { useLiveUser } from "hooks/useLiveUser";
import ThumbnailSelector from "components/thumbnail-selector";
import { useCaptureVideoFrame } from "hooks/useCaptureVideoFrame";
import { useWindowDimensions, ScrollView } from "react-native";

const Left = ({ children }: { children: React.ReactNode }) => {
  return (
    <View f={2} fg={2} fb={0}>
      {children}
    </View>
  );
};

const Right = ({ children }: { children: React.ReactNode }) => {
  return (
    <View f={6} fb={0} fg={6}>
      {children}
    </View>
  );
};

export default function CreateLivestream() {
  const dispatch = useAppDispatch();
  const toast = useToastController();
  const userIsLive = useLiveUser();
  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(false);
  const [customThumbnail, setCustomThumbnail] = useState<Blob | undefined>(
    undefined,
  );
  const profile = useAppSelector(selectUserProfile);
  const newLivestream = useAppSelector(selectNewLivestream);
  const captureFrame = useCaptureVideoFrame();
  const { width, height } = useWindowDimensions();

  // Responsive layout logic
  const isTall = height > 700;
  const isWide = width > 800;
  const useTwoColumns = !isTall && isWide;
  const useScroll = !isTall && !isWide;

  useEffect(() => {
    if (newLivestream?.record) {
      toast.show("Livestream announced", {
        message: newLivestream.record.title,
      });
      setTitle("");
      setCustomThumbnail(undefined);
    }
  }, [newLivestream?.record]);
  useEffect(() => {
    if (newLivestream?.error) {
      toast.show("Error creating livestream", {
        message: newLivestream.error,
      });
    }
  }, [newLivestream?.error]);
  const disabled = !userIsLive || loading || title === "";

  function FormFields({
    includeThumbnail = false,
  }: {
    includeThumbnail?: boolean;
  }) {
    // Determine if we are in single column mode
    const isSingleColumn = !useTwoColumns;
    return (
      <>
        <Label asChild={true} display="flex">
          <View flexDirection="row">
            <Left>
              <Paragraph pb="$2">Streamer</Paragraph>
            </Left>
            <Right>
              <Paragraph pb="$2">@{profile?.handle}</Paragraph>
            </Right>
          </View>
        </Label>
        <Label asChild={true}>
          <View flexDirection="row">
            <Left>
              <Paragraph pb="$2">Title</Paragraph>
            </Left>
            <Right>
              <View w="100%" ai={isSingleColumn ? "center" : undefined}>
                <TextArea
                  value={title}
                  onChangeText={setTitle}
                  size="$4"
                  minHeight={100}
                  maxLength={140}
                  w={isSingleColumn ? "100%" : undefined}
                  maxWidth={isSingleColumn ? 600 : undefined}
                />
              </View>
            </Right>
          </View>
        </Label>
        {includeThumbnail && (
          <Label asChild={true}>
            <View flexDirection="row" ai="flex-start">
              <Left>
                <Paragraph pb={0} lineHeight={18}>
                  Custom Thumbnail
                </Paragraph>
                <Paragraph pb="$2" lineHeight={18} fontSize={14}>
                  (Optional)ss
                </Paragraph>
              </Left>
              <Right>
                <View w="100%" ai={isSingleColumn ? "center" : undefined}>
                  <View maxWidth={600} w="100%">
                    <ThumbnailSelector
                      onThumbnailSelected={setCustomThumbnail}
                    />
                  </View>
                </View>
              </Right>
            </View>
          </Label>
        )}
        <View gap="$2" w="100%" ai={isSingleColumn ? "center" : undefined}>
          <Button
            disabled={disabled}
            opacity={disabled ? 0.5 : 1}
            w={isSingleColumn ? "100%" : "100%"}
            maxWidth={isSingleColumn ? 600 : undefined}
            size="$4"
            onPress={async () => {
              setLoading(true);
              try {
                // If no custom thumbnail is provided and we're on web, try to capture one from the video
                let thumbnailToUse = customThumbnail;
                if (!thumbnailToUse && isWeb && captureFrame) {
                  const capturedFrame = await captureFrame(1280, 0.85);
                  if (capturedFrame) {
                    thumbnailToUse = capturedFrame;
                  }
                }

                await dispatch(
                  createLivestreamRecord({
                    title,
                    customThumbnail: thumbnailToUse,
                  }),
                );
              } catch (error) {
                console.error("Error creating livestream:", error);
                toast.show("Error creating livestream", {
                  message: String(error),
                });
              } finally {
                setLoading(false);
              }
            }}
          >
            {buttonText(loading, userIsLive)}
          </Button>
        </View>
      </>
    );
  }

  function ThumbnailSection() {
    return (
      <View gap="$2" ai="flex-start" w="100%">
        <Paragraph pb={0} lineHeight={18}>
          Custom Thumbnail (Optional)
        </Paragraph>
        <View maxWidth={600} w="100%">
          <ThumbnailSelector onThumbnailSelected={setCustomThumbnail} />
        </View>
      </View>
    );
  }

  if (useScroll) {
    return (
      <ScrollView contentContainerStyle={{ flexGrow: 1 }}>
        <View
          f={1}
          ai="stretch"
          jc="center"
          gap="$4"
          w="100%"
          p="$4"
          maxWidth={900}
        >
          <FormFields includeThumbnail={true} />
        </View>
      </ScrollView>
    );
  }

  if (!useTwoColumns) {
    return (
      <View
        f={1}
        ai="stretch"
        jc="center"
        gap="$4"
        w="100%"
        p="$4"
        maxWidth={900}
      >
        <FormFields includeThumbnail={true} />
      </View>
    );
  }

  // Two-column layout
  return (
    <View
      f={1}
      ai="stretch"
      jc="center"
      gap="$4"
      w="100%"
      p="$4"
      maxWidth={900}
    >
      <View flexDirection="row" gap="$8" w="100%">
        <View f={2} minWidth={0} gap="$4">
          <FormFields includeThumbnail={false} />
        </View>
        <View f={1} minWidth={0} gap="$4" ai="flex-start">
          <ThumbnailSection />
        </View>
      </View>
    </View>
  );
}

const buttonText = (loading: boolean, userIsLive: boolean) => {
  if (loading) {
    return "Loading...";
  }
  if (!userIsLive) {
    return "Waiting for stream to start...";
  }
  return "Announce Livestream!";
};
