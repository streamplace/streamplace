import { TriangleAlert } from "lucide-react-native";
import { useEffect, useRef } from "react";
import { Platform } from "react-native";
import { StreamplaceAgent } from "streamplace";
import { useToast } from "../components/ui/toast";
import { useUrl } from "../streamplace-store/streamplace-store";
import { checkClockDrift, syncTimeWithServer } from "./time-sync";

export function useTimeSync() {
  const url = useUrl();
  const t = useToast();
  const hasShownWarning = useRef(false);

  useEffect(() => {
    const checkTime = async () => {
      if (Platform.OS !== "web") {
        return;
      }
      try {
        const agent = new StreamplaceAgent(url);
        const start = new Date().getTime();
        const response = await agent.place.stream.server.getServerTime();
        const roundTripLatency = new Date().getTime() - start;
        const serverTime = response.data.serverTime;

        // always sync with server time
        syncTimeWithServer(serverTime, roundTripLatency / 2);

        const driftInfo = checkClockDrift(serverTime);

        // only show warning if drift is significant
        if (driftInfo.hasDrift && !hasShownWarning.current) {
          hasShownWarning.current = true;
          t.show(
            "Clock drift detected!",
            `Your device clock is ${driftInfo.driftSeconds}s off from server time. Please sync your system clock to avoid issues.`,
            {
              variant: "info",
              iconLeft: TriangleAlert,
              duration: 25,
            },
          );
          console.log(
            `time sync applied: offset ${driftInfo.driftMs}ms. Date() calls will now use server time.`,
          );
        }
      } catch (error) {
        console.error("failed to sync time with server:", error);
      }
    };

    checkTime();

    const interval = setInterval(checkTime, 1800000); // every 30m

    return () => clearInterval(interval);
  }, [url, t]);
}
