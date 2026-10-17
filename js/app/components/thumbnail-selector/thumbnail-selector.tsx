import { Button, Text, useTheme } from "@streamplace/components";
import { colors, scrims } from "@streamplace/components/src/lib/theme/tokens";
import { Image } from "expo-image";
import { useCallback, useEffect, useRef, useState } from "react";
import { Platform, TouchableOpacity, View } from "react-native";
import { ThumbnailSelectorProps } from "./shared";

export default function ThumbnailSelector({
  onThumbnailSelected,
  thumbnailUrl,
}: ThumbnailSelectorProps) {
  const { theme } = useTheme();
  const [selectedImage, setSelectedImage] = useState<string | null>(
    thumbnailUrl || null,
  );

  const isWeb = Platform.OS === "web";

  useEffect(() => {
    if (thumbnailUrl) {
      setSelectedImage(thumbnailUrl);
    }
  }, [thumbnailUrl]);

  const revokeObjectURL = useCallback(
    (imageUrl: string | null) => {
      if (isWeb && imageUrl && imageUrl.startsWith("blob:")) {
        URL.revokeObjectURL(imageUrl);
      }
    },
    [isWeb],
  );

  useEffect(() => {
    return () => {
      revokeObjectURL(selectedImage);
    };
  }, [selectedImage, revokeObjectURL]);

  const clearThumbnail = useCallback(() => {
    revokeObjectURL(selectedImage);
    setSelectedImage(null);
    onThumbnailSelected(undefined);
  }, [onThumbnailSelected, selectedImage, revokeObjectURL]);

  const [showWebCamera, setShowWebCamera] = useState(false);
  const [webCameraStream, setWebCameraStream] = useState<MediaStream | null>(
    null,
  );
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (showWebCamera && videoRef.current && webCameraStream) {
      videoRef.current.srcObject = webCameraStream;
    }
  }, [showWebCamera, webCameraStream]);

  const captureWebFrame = useCallback(() => {
    if (videoRef.current && canvasRef.current) {
      const video = videoRef.current;
      const canvas = canvasRef.current;

      canvas.width = video.videoWidth;
      canvas.height = video.videoHeight;

      const ctx = canvas.getContext("2d");
      if (ctx) {
        ctx.drawImage(video, 0, 0, canvas.width, canvas.height);

        canvas.toBlob(
          (blob) => {
            if (blob) {
              const imageUrl = URL.createObjectURL(blob);
              setSelectedImage(imageUrl);
              onThumbnailSelected(blob);

              if (video.srcObject) {
                const stream = video.srcObject as MediaStream;
                const tracks = stream.getTracks();
                tracks.forEach((track) => track.stop());
              }

              setShowWebCamera(false);
              setWebCameraStream(null);
            }
          },
          "image/jpeg",
          0.85,
        );
      }
    }
  }, [onThumbnailSelected]);

  const startWebCamera = useCallback(async () => {
    try {
      if (!navigator.mediaDevices) {
        throw new Error("Media devices API not available in this browser");
      }

      const stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: "environment" },
      });

      setShowWebCamera(true);
      setWebCameraStream(stream);
    } catch (error) {
      console.error("Error accessing camera:", error);

      // Fallback to file input if camera access fails
      if (fileInputRef.current) {
        fileInputRef.current.click();
      }
    }
  }, []);

  const pickImage = useCallback(() => {
    if (fileInputRef.current) {
      fileInputRef.current.click();
    }
  }, []);

  const takePhoto = useCallback(async () => {
    if (isWeb) {
      await startWebCamera();
    }
  }, [startWebCamera, isWeb]);

  const handleFileInputChange = useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      try {
        const files = event.target.files;
        if (files && files.length > 0) {
          const file = files[0];
          const blob = new Blob([file], { type: file.type });
          const imageUrl = URL.createObjectURL(blob);

          setSelectedImage(imageUrl);
          onThumbnailSelected(blob);
          event.target.value = "";
        }
      } catch (error) {
        console.error("Error processing file:", error);
      }
    },
    [onThumbnailSelected],
  );

  return (
    <View>
      {showWebCamera && isWeb ? (
        <View
          style={[
            {
              position: "relative",
              height: 300,
              width: "100%",
              backgroundColor: theme.colors.surface2,
              borderRadius: 8,
            },
          ]}
        >
          {/* Web camera video element */}
          <video
            ref={videoRef}
            autoPlay
            playsInline
            style={{
              width: "100%",
              height: "100%",
              objectFit: "cover",
              borderRadius: 8,
            }}
          />
          <canvas ref={canvasRef} style={{ display: "none" }} />

          {/* Camera controls */}
          <View
            style={[
              {
                position: "absolute",
                bottom: 10,
                width: "100%",
                flexDirection: "row",
                justifyContent: "center",
                gap: 8,
              },
            ]}
          >
            <TouchableOpacity
              style={[
                {
                  backgroundColor: scrims.dark,
                  borderRadius: 8,
                  paddingHorizontal: 16,
                  paddingVertical: 12,
                  flexDirection: "row",
                  alignItems: "center",
                  gap: 8,
                },
              ]}
              onPress={captureWebFrame}
            >
              <Text style={[{ fontSize: 16 }]}>📷</Text>
              <Text style={[{ color: colors.white, fontWeight: "600" }]}>
                Capture
              </Text>
            </TouchableOpacity>
            <TouchableOpacity
              style={[
                {
                  backgroundColor: scrims.dark,
                  borderRadius: 8,
                  paddingHorizontal: 16,
                  paddingVertical: 12,
                  flexDirection: "row",
                  alignItems: "center",
                  gap: 8,
                },
              ]}
              onPress={() => {
                // Stop the camera stream
                if (videoRef.current && videoRef.current.srcObject) {
                  const stream = videoRef.current.srcObject as MediaStream;
                  const tracks = stream.getTracks();
                  tracks.forEach((track) => track.stop());
                }
                setShowWebCamera(false);
                setWebCameraStream(null);
              }}
            >
              <Text style={[{ fontSize: 16 }]}>×</Text>
              <Text style={[{ color: colors.white, fontWeight: "600" }]}>
                Cancel
              </Text>
            </TouchableOpacity>
          </View>
        </View>
      ) : selectedImage ? (
        <View style={[{ position: "relative" }]}>
          <Image
            source={{ uri: selectedImage }}
            style={[
              {
                width: "100%",
                height: 150,
                borderRadius: 8,
              },
            ]}
            contentFit="cover"
          />
          <TouchableOpacity
            style={[
              {
                position: "absolute",
                top: 5,
                right: 5,
                width: 24,
                height: 24,
                borderRadius: 12,
                backgroundColor: scrims.dark,
                alignItems: "center",
                justifyContent: "center",
              },
            ]}
            onPress={clearThumbnail}
          >
            <Text style={[{ fontSize: 16, color: colors.white }]}>×</Text>
          </TouchableOpacity>
        </View>
      ) : (
        <View
          style={[
            {
              height: 150,
              width: "100%",
              backgroundColor: theme.colors.surface2,
              borderRadius: 8,
              justifyContent: "center",
              alignItems: "center",
            },
          ]}
        >
          <Text style={[{ color: theme.colors.text1 }]}>
            No thumbnail selected
          </Text>
        </View>
      )}

      {!showWebCamera && (
        <View style={[{ flexDirection: "row", gap: 8, marginTop: 8 }]}>
          {/* Two equal utility actions → secondary (tonal), so neither reads as
              a competing Paper primary. */}
          <View style={{ flex: 1 }}>
            <Button
              variant="secondary"
              onPress={pickImage}
              leftIcon={<Text style={{ fontSize: 16 }}>🖼️</Text>}
            >
              Choose Image
            </Button>
          </View>
          <View style={{ flex: 1 }}>
            <Button
              variant="secondary"
              onPress={takePhoto}
              leftIcon={<Text style={{ fontSize: 16 }}>📷</Text>}
            >
              Take Photo
            </Button>
          </View>
        </View>
      )}

      {/* Hidden file input */}
      {isWeb && (
        <input
          type="file"
          accept="image/*"
          ref={fileInputRef}
          onChange={handleFileInputChange}
          style={{ display: "none" }}
        />
      )}
    </View>
  );
}
