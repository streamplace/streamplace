import { ContentMetadata, ContentMetadataForm } from "@streamplace/components";
import { useToastController } from "@tamagui/toast";
import ThumbnailSelector from "components/thumbnail-selector";
import {
  createLivestreamRecord,
  selectNewLivestream,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import { useCaptureVideoFrame } from "hooks/useCaptureVideoFrame";
import { useLiveUser } from "hooks/useLiveUser";
import { useEffect, useState } from "react";
import { useWindowDimensions } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import {
  Button,
  H3,
  Label,
  Paragraph,
  ScrollView,
  TextArea,
  View,
  XStack,
  YStack,
  isWeb,
} from "tamagui";

export default function CreateLivestream() {
  const dispatch = useAppDispatch();
  const toast = useToastController();
  const userIsLive = useLiveUser();
  const [title, setTitle] = useState("");
  const [loading, setLoading] = useState(false);
  const [customThumbnail, setCustomThumbnail] = useState<Blob | undefined>(
    undefined,
  );
  const [contentMetadata, setContentMetadata] = useState<ContentMetadata>({
    contentWarnings: { warnings: [] },
    distributionPolicy: {
      deleteAfter: undefined, // No expiration means forever
    },
    contentRights: {},
  });
  const [showMetadata, setShowMetadata] = useState(false);
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
      setContentMetadata({
        contentWarnings: { warnings: [] },
        distributionPolicy: {
          deleteAfter: undefined, // No expiration means forever
        },
        contentRights: {},
      });
      setShowMetadata(false);
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

  if (showMetadata) {
    return (
      <YStack f={1}>
        {/* Header with Back Button */}
        <XStack
          paddingHorizontal="$3"
          paddingVertical="$2"
          backgroundColor="$background"
          borderBottomWidth={1}
          borderBottomColor="$gray4"
          alignItems="center"
          justifyContent="center"
          position="relative"
        >
          <YStack alignItems="center">
            <H3 fontSize="$4">Content Metadata</H3>
            <Paragraph fontSize="$1" color="$gray11">
              Optional metadata for your livestream
            </Paragraph>
          </YStack>

          <View position="absolute" left="$3">
            <Button
              size="$2"
              variant="outlined"
              onPress={() => setShowMetadata(false)}
              icon={<Paragraph fontSize="$2">←</Paragraph>}
            >
              Back to Basic Info
            </Button>
          </View>
        </XStack>

        {/* Scrollable Content */}
        <ScrollView style={{ flex: 1 }} showsVerticalScrollIndicator={false}>
          <YStack f={1} gap="$2" pb="$3">
            {/* Content Metadata Form */}
            <ContentMetadataForm
              onMetadataChange={setContentMetadata}
              initialMetadata={contentMetadata}
              showUpdateButton={true}
            />

            {/* Submit Button */}
            <View p="$2" alignItems="center">
              <Button
                disabled={disabled}
                opacity={disabled ? 0.5 : 1}
                size="$3"
                w="100%"
                maxWidth={400}
                onPress={handleSubmit}
              >
                {buttonText}
              </Button>
            </View>
          </YStack>
        </ScrollView>
      </YStack>
    );
  }

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
          <View w="100%" alignItems="center" mt="$4" gap="$3">
            <Button
              size="$4"
              variant="outlined"
              onPress={() => setShowMetadata(true)}
            >
              Add Content Metadata
            </Button>
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
