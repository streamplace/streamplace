import { useEffect, useRef } from "react";
import { View } from "tamagui";
import QRCode from "qrcode";

const toArrayBuffer = (value: number) => {
  const ab = new ArrayBuffer(8);
  const view = new DataView(ab);
  view.setFloat64(0, value);
  return ab;
};

export default function AVSyncScreen() {
  useEffect(() => {
    let interval: NodeJS.Timeout | null = null;
    async function initQuiet() {
      const quiet = await import("quietjs-bundle");
      quiet.addReadyCallback(() => {
        const transmitter = quiet.transmitter({ profile: "audible" });
        interval = setInterval(() => {
          const ab = toArrayBuffer(Date.now());
          transmitter.transmit(ab);
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
        QRCode.toCanvas(
          canvasRef.current,
          [
            {
              data: toArrayBuffer(Date.now()) as Uint8Array,
              mode: "byte",
            },
          ],
          function (error) {
            if (error) console.error(error);
          },
        );
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
      <canvas
        ref={canvasRef}
        width={600}
        height={600}
        style={{ transform: "scale(3)", imageRendering: "pixelated" }}
      />
    </View>
  );
}
