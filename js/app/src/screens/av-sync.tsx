import { useEffect, useRef } from "react";
import { View } from "tamagui";
import QRCode from "qrcode";
import { Countdown } from "components";
import { str2ab } from "quietjs-bundle";
import { QUIET_PROFILE } from "components/player/av-sync";

// screen that displays timestamp as a QR code and encodes timestamp in audio
// so we can measure sync between them

export default function AVSyncScreen() {
  useEffect(() => {
    let interval: NodeJS.Timeout | null = null;
    async function initQuiet() {
      const quiet = await import("quietjs-bundle");
      quiet.addReadyCallback(() => {
        const transmitter = quiet.transmitter({
          profile: QUIET_PROFILE,
        });
        interval = setInterval(() => {
          transmitter.transmit(str2ab(`${Date.now()}`));
        }, 1000);
      });
    }
    initQuiet();
    return () => {
      if (interval) {
        clearInterval(interval);
      }
    };
  }, []);

  const canvasRef = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    let stopped = false;
    const frame = () => {
      if (stopped) {
        return;
      }
      if (canvasRef.current) {
        QRCode.toCanvas(canvasRef.current, `${Date.now()}`, function (error) {
          if (error) console.error(error);
        });
      }
      requestAnimationFrame(frame);
    };
    frame();
    return () => {
      stopped = true;
    };
  }, []);

  return (
    <View flex={1} justifyContent="center" alignItems="center">
      <View f={1} justifyContent="center" alignItems="center">
        <Countdown from="now" />
      </View>
      <View height={348} f={1} justifyContent="center" alignItems="center">
        <canvas
          ref={canvasRef}
          style={{ transform: "scale(3)", imageRendering: "pixelated" }}
        />
      </View>
    </View>
  );
}
