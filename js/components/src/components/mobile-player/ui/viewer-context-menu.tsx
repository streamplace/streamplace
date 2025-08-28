import { useRootContext } from "@rn-primitives/dropdown-menu";
import { Settings } from "lucide-react-native";
import { Platform, View } from "react-native";
import { colors } from "../../../lib/theme";
import { useLivestreamStore } from "../../../livestream-store";
import { PlayerProtocol, usePlayerStore } from "../../../player-store/";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContentWithoutPortal,
  DropdownMenuGroup,
  DropdownMenuInfo,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
  Text,
} from "../../ui";

export function ContextMenu({
  dropdownPortalContainer,
}: {
  dropdownPortalContainer?: any;
}) {
  const quality = usePlayerStore((x) => x.selectedRendition);
  const setQuality = usePlayerStore((x) => x.setSelectedRendition);
  const qualities = useLivestreamStore((x) => x.renditions);

  const protocol = usePlayerStore((x) => x.protocol);
  const setProtocol = usePlayerStore((x) => x.setProtocol);

  const debugInfo = usePlayerStore((x) => x.showDebugInfo);
  const setShowDebugInfo = usePlayerStore((x) => x.setShowDebugInfo);

  const livestream = useLivestreamStore((x) => x.livestream);
  const setReportModalOpen = usePlayerStore((x) => x.setReportModalOpen);
  const setReportSubject = usePlayerStore((x) => x.setReportSubject);

  const lowLatency = protocol === "webrtc";
  const setLowLatency = (value: boolean) => {
    setProtocol(value ? PlayerProtocol.WEBRTC : PlayerProtocol.HLS);
  };

  // are we on mobile? then do dropdowns
  const isMobile = Platform.OS === "ios" || Platform.OS === "android";

  // dummy portal for mobile
  const Portal = isMobile ? View : DropdownMenuPortal;

  // render the responsive version on mobile as we can't fullscreen there
  const DropdownMenuContent = isMobile
    ? ResponsiveDropdownMenuContent
    : DropdownMenuContentWithoutPortal;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger>
        <Settings color={colors.gray[200]} />
      </DropdownMenuTrigger>
      <Portal container={dropdownPortalContainer}>
        <DropdownMenuContent side="top" align="end">
          <DropdownMenuGroup title="Resolution">
            <DropdownMenuRadioGroup value={quality} onValueChange={setQuality}>
              <DropdownMenuRadioItem value="source">
                <Text>Source (Original Quality)</Text>
              </DropdownMenuRadioItem>
              {qualities.map((r) => (
                <DropdownMenuRadioItem value={r.name}>
                  <Text>{r.name}</Text>
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuGroup>
          <DropdownMenuGroup title="Advanced">
            <DropdownMenuCheckboxItem
              checked={lowLatency}
              onCheckedChange={() => setLowLatency(!lowLatency)}
            >
              <Text>Low Latency</Text>
            </DropdownMenuCheckboxItem>
          </DropdownMenuGroup>
          <DropdownMenuInfo description="Reduces the delay between video and chat for a more real-time experience." />
          <DropdownMenuGroup>
            <DropdownMenuCheckboxItem
              checked={debugInfo}
              onCheckedChange={() => setShowDebugInfo(!debugInfo)}
            >
              <Text>Show Debug Info</Text>
            </DropdownMenuCheckboxItem>
          </DropdownMenuGroup>
          <DropdownMenuGroup title="Report">
            <ReportButton
              livestream={livestream}
              setReportModalOpen={setReportModalOpen}
              setReportSubject={setReportSubject}
            />
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </Portal>
    </DropdownMenu>
  );
}

export function ReportButton({
  livestream,
  setReportModalOpen,
  setReportSubject,
}) {
  const { onOpenChange } = useRootContext();
  return (
    <DropdownMenuItem
      onPress={() => {
        if (!livestream) return;
        onOpenChange?.(false);
        setReportModalOpen(true);
        setReportSubject({
          $type: "com.atproto.repo.strongRef",
          uri: livestream.uri,
          cid: livestream.cid,
        });
      }}
    >
      <Text>Report Livestream...</Text>
    </DropdownMenuItem>
  );
}
