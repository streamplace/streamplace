import { useCallback, useEffect, useState } from "react";
import { Button, Image, Text, View } from "tamagui";
import { isWeb } from "tamagui";
import { Platform } from "react-native";
import * as ImagePicker from "expo-image-picker";
import { Camera, Image as ImageIcon, X } from "@tamagui/lucide-icons";
import { captureVideoFrame, findVideoElement } from "utils/videoCapture";

interface ThumbnailSelectorProps {
  onThumbnailSelected: (blob: Blob | undefined) => void;
  thumbnailUrl?: string;
}

export default function ThumbnailSelector({
  onThumbnailSelected,
  thumbnailUrl,
}: ThumbnailSelectorProps) {
  const [selectedImage, setSelectedImage] = useState<string | null>(
    thumbnailUrl || null,
  );
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (thumbnailUrl) {
      setSelectedImage(thumbnailUrl);
    }
  }, [thumbnailUrl]);

  const pickImage = useCallback(async () => {
    setLoading(true);
    try {
      // Request permissions first
      if (Platform.OS !== "web") {
        const { status } =
          await ImagePicker.requestMediaLibraryPermissionsAsync();
        if (status !== "granted") {
          alert("Sorry, we need camera roll permissions to make this work!");
          return;
        }
      }

      // Launch image picker
      const result = await ImagePicker.launchImageLibraryAsync({
        mediaTypes: ImagePicker.MediaTypeOptions.Images,
        allowsEditing: true,
        aspect: [16, 9],
        quality: 0.8,
      });

      if (!result.canceled) {
        const imageUri = result.assets[0].uri;
        setSelectedImage(imageUri);

        const response = await fetch(imageUri);
        const blob = await response.blob();
        onThumbnailSelected(blob);
      } else {
        // User canceled
        console.log("Image selection canceled");
      }
    } catch (error) {
      console.error("Error picking image:", error);
    } finally {
      setLoading(false);
    }
  }, [onThumbnailSelected]);

  // Capture a frame from the video stream
  const captureFrame = useCallback(async () => {
    if (!isWeb) {
      alert("Capturing from stream is only available on web");
      return;
    }

    setLoading(true);
    try {
      const videoElement = findVideoElement();
      if (!videoElement) {
        alert("No video stream found");
        return;
      }

      const blob = await captureVideoFrame(videoElement, 1280, 0.85);
      const imageUrl = URL.createObjectURL(blob);
      setSelectedImage(imageUrl);
      onThumbnailSelected(blob);
    } catch (error) {
      console.error("Error capturing frame:", error);
      alert("Failed to capture frame from video");
    } finally {
      setLoading(false);
    }
  }, [onThumbnailSelected]);

  // Clear the selected thumbnail
  const clearThumbnail = useCallback(() => {
    setSelectedImage(null);
    onThumbnailSelected(undefined);
  }, [onThumbnailSelected]);

  return (
    <View>
      <Text pb="$2">Thumbnail (optional)</Text>

      {selectedImage ? (
        <View position="relative">
          <Image
            source={{ uri: selectedImage }}
            width="100%"
            height={150}
            objectFit="cover"
            borderRadius="$2"
          />
          <Button
            position="absolute"
            top={5}
            right={5}
            size="$2"
            circular
            icon={<X size={16} />}
            onPress={clearThumbnail}
          />
        </View>
      ) : (
        <View
          height={150}
          width="100%"
          backgroundColor="$backgroundHover"
          borderRadius="$2"
          justifyContent="center"
          alignItems="center"
        >
          <Text color="$color">No thumbnail selected</Text>
        </View>
      )}

      <View flexDirection="row" gap="$2" mt="$2">
        <Button
          flex={1}
          icon={<ImageIcon size={16} />}
          onPress={pickImage}
          disabled={loading}
        >
          {loading ? "Loading..." : "Choose Image"}
        </Button>
        {/* TODO: Re-enable this when we have camera working */}
        {/* {isWeb && (
          <Button
            flex={1}
            icon={<Camera size={16} />}
            onPress={captureFrame}
            disabled={loading}
          >
            Capture Frame
          </Button>
        )} */}
      </View>
    </View>
  );
}
