import { Camera, CameraView, Subscription } from "expo-camera";
import { Linking, Platform } from "react-native";

// what a browser's QR login code encodes: https://<node>/login/qr/<id>
const QR_LOGIN_PATH = /\/login\/qr\/([^/?#]+)/;

let subscription: Subscription | undefined;

// Scan a browser's QR login code with the OS's scanner (Google code scanner on
// Android, DataScanner on iOS 16+), calling onScan with its id. Undefined where
// the device has no such scanner.
export const scanQrLogin = CameraView.isModernBarcodeScannerAvailable
  ? async (onScan: (id: string) => void) => {
      // the Google code scanner needs no camera permission; DataScanner does
      if (Platform.OS === "ios") {
        const permission = await Camera.requestCameraPermissionsAsync();
        if (!permission.granted) {
          if (!permission.canAskAgain) Linking.openSettings();
          return;
        }
      }
      // iOS reports nothing when its scanner is swiped away, so a listener can
      // outlive its scan
      subscription?.remove();
      subscription = CameraView.onModernBarcodeScanned(({ data }) => {
        const id = data.match(QR_LOGIN_PATH)?.[1];
        // iOS keeps scanning, so other codes are skipped until ours shows up
        if (!id) return;
        subscription?.remove();
        subscription = undefined;
        CameraView.dismissScanner();
        onScan(id);
      });
      try {
        await CameraView.launchScanner({ barcodeTypes: ["qr"] });
      } catch (e) {
        if (
          (e as { code?: string }).code !== "ERR_BARCODE_SCANNING_CANCELLED"
        ) {
          throw e;
        }
      }
    }
  : undefined;
