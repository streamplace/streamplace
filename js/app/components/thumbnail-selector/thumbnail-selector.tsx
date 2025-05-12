import { useCallback, useEffect, useRef, useState } from "react";
import { Button, Image, Text, View, isWeb } from "tamagui";
import { Platform } from "react-native";
import * as ImagePicker from "expo-image-picker";
import { Camera, Image as ImageIcon, X } from "@tamagui/lucide-icons";
import { ThumbnailSelectorProps } from "./shared";

export default function ThumbnailSelector({
  onThumbnailSelected,
  thumbnailUrl,
}: ThumbnailSelectorProps) {
  const [selectedImage, setSelectedImage] = useState<string | null>(
    thumbnailUrl || null,
  );

  useEffect(() => {
    if (thumbnailUrl) {
      setSelectedImage(thumbnailUrl);
    }
  }, [thumbnailUrl]);

  const revokeObjectURL = useCallback((imageUrl: string | null) => {
    if (isWeb && imageUrl && imageUrl.startsWith("blob:")) {
      URL.revokeObjectURL(imageUrl);
    }
  }, []);

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

  const galleryOptions: ImagePicker.ImagePickerOptions = {
    mediaTypes: "images",
    allowsEditing: true,
    aspect: [16, 9] as [number, number],
    quality: 0.8,
  };

  const cameraOptions: ImagePicker.ImagePickerOptions = {
    mediaTypes: "images",
    allowsEditing: true,
    aspect: [16, 9] as [number, number],
    quality: 0.8,
    cameraType: ImagePicker.CameraType.back,
  };

  const processImageResult = useCallback(
    async (result: ImagePicker.ImagePickerResult, source: string) => {
      if (!result.canceled) {
        const imageUri = result.assets[0].uri;
        setSelectedImage(imageUri);

        const response = await fetch(imageUri);
        const blob = await response.blob();
        onThumbnailSelected(blob);
      } else {
        // User canceled
        console.log(`${source} selection canceled`);
      }
    },
    [onThumbnailSelected],
  );

  const pickImage = useCallback(async () => {
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

      const result = await ImagePicker.launchImageLibraryAsync(galleryOptions);

      await processImageResult(result, "Image");
    } catch (error) {
      console.error("Error picking image:", error);
    }
  }, [processImageResult, galleryOptions]);

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
      console.log("Assigned stream to video element");
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
        console.log("Media devices API not available in this browser");
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

  const takePhoto = useCallback(async () => {
    try {
      if (isWeb) {
        await startWebCamera();
      } else {
        // On native platforms, use ImagePicker
        // Request camera permissions first
        const { status } = await ImagePicker.requestCameraPermissionsAsync();

        if (status !== "granted") {
          alert("Sorry, we need camera permissions to make this work!");
          return;
        }

        const result = await ImagePicker.launchCameraAsync(cameraOptions);

        await processImageResult(result, "Camera");
      }
    } catch (error) {
      console.error("Error taking picture:", error);
      alert(`Error opening camera: ${error.message}`);
    }
  }, [processImageResult, startWebCamera]);

  const handleFileInputChange = useCallback(
    async (event: React.ChangeEvent<HTMLInputElement>) => {
      try {
        const files = event.target.files;
        if (files && files.length > 0) {
          const file = files[0];
          console.log("File selected:", file);

          const imageUrl = URL.createObjectURL(file);
          setSelectedImage(imageUrl);

          const blob = await new Response(file).blob();
          onThumbnailSelected(blob);
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
          position="relative"
          height={300}
          width="100%"
          backgroundColor="$backgroundHover"
          borderRadius="$2"
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
            position="absolute"
            bottom={10}
            width="100%"
            flexDirection="row"
            justifyContent="center"
            gap="$2"
          >
            <Button
              icon={<Camera size={16} />}
              onPress={captureWebFrame}
              backgroundColor="transparent"
            >
              Capture
            </Button>
            <Button
              icon={<X size={16} />}
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
              backgroundColor="transparent"
            >
              Cancel
            </Button>
          </View>
        </View>
      ) : selectedImage ? (
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

      {!showWebCamera && (
        <View flexDirection="row" gap="$2" mt="$2">
          <Button flex={1} icon={<ImageIcon size={16} />} onPress={pickImage}>
            Choose Image
          </Button>
          <Button flex={1} icon={<Camera size={16} />} onPress={takePhoto}>
            Take Photo
          </Button>
        </View>
      )}

      {/* Hidden file input for web camera capture fallback */}
      {isWeb && (
        <input
          type="file"
          accept="image/*"
          capture="environment"
          ref={fileInputRef}
          onChange={handleFileInputChange}
          style={{ display: "none" }}
        />
      )}
    </View>
  );
}
