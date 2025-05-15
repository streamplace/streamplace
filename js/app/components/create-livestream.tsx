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
  const { width } = useWindowDimensions();

  // Responsive layout logic
  const isWide = width > 1020;
  const useTwoColumns = isWide;

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

  const handleSubmit = async () => {
    setLoading(true);
    try {
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
  };

  const buttonText = loading
    ? "Loading..."
    : !userIsLive
      ? "Waiting for stream to start..."
      : "Announce Livestream!";

  return (
    <ScrollView
      style={{ width: "60%" }}
      contentContainerStyle={{
        flexGrow: 1,
        justifyContent: "flex-start",
        paddingVertical: 40,
      }}
      showsVerticalScrollIndicator={false}
    >
      <View
        flexDirection={useTwoColumns ? "row" : "column"}
        gap={useTwoColumns ? 48 : 16}
        w="100%"
        maxWidth={useTwoColumns ? 900 : undefined}
        alignSelf="center"
        p="$4"
        alignItems={useTwoColumns ? "flex-start" : "stretch"}
        justifyContent="center"
      >
        {/* Left column: labels and fields */}
        <View f={2} minWidth={0} gap="$3" w={useTwoColumns ? 500 : "100%"}>
          <Label asChild={true} display="flex">
            <View flexDirection="row" alignItems="center" w="100%">
              <Paragraph pb="$2" minWidth={100} textAlign="left">
                Streamer
              </Paragraph>
              <Paragraph pb="$2" fontWeight="bold">
                @{profile?.handle}
              </Paragraph>
            </View>
          </Label>
          <Label asChild={true}>
            <View flexDirection="row" alignItems="center" w="100%">
              <Paragraph pb="$2" minWidth={100} textAlign="left">
                Title
              </Paragraph>
              <View flex={1}>
                <TextArea
                  id="livestream-title"
                  value={title}
                  onChangeText={setTitle}
                  size="$4"
                  minHeight={100}
                  maxLength={140}
                  w="100%"
                />
              </View>
            </View>
          </Label>
          <View w="100%" alignItems="center" mt="$4">
            <Button
              disabled={disabled}
              opacity={disabled ? 0.5 : 1}
              size="$4"
              w="100%"
              onPress={handleSubmit}
            >
              {buttonText}
            </Button>
          </View>
        </View>
        {/* Right column: thumbnail */}
        <View
          f={1}
          minWidth={0}
          gap="$4"
          alignItems="center"
          justifyContent="flex-start"
          w={useTwoColumns ? 400 : "100%"}
          style={{
            marginTop: 12,
            ...(useTwoColumns ? {} : { marginLeft: 40 }),
          }}
        >
          <Label asChild={true}>
            <View flexDirection="column" alignItems="center" w="100%">
              <Paragraph pb={0} lineHeight={18} fontWeight="bold" mb="$2">
                Custom Thumbnail (Optional)
              </Paragraph>
              <View maxWidth={400} w="100%">
                <ThumbnailSelector onThumbnailSelected={setCustomThumbnail} />
              </View>
            </View>
          </Label>
        </View>
      </View>
    </ScrollView>
  );
}
