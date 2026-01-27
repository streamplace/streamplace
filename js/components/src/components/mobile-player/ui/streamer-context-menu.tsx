import { ChevronRight, Cog } from "lucide-react-native";
import { useEffect, useState } from "react";
import Animated, {
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withDelay,
  withSequence,
  withTiming,
} from "react-native-reanimated";
import { useLivestreamInfo, zero } from "../../..";
import { usePlayerStore } from "../../../player-store";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
  Text,
  useTheme,
} from "../../ui";

export function StreamContextMenu({
  dropdownPortalContainer,
}: {
  dropdownPortalContainer?: string;
}) {
  const th = useTheme();
  const debugInfo = usePlayerStore((x) => x.showDebugInfo);
  const setShowDebugInfo = usePlayerStore((x) => x.setShowDebugInfo);
  const { toggleStopStream } = useLivestreamInfo();
  const ingest = usePlayerStore((x) => x.ingestConnectionState);
  const isLive = ingest !== null && ingest !== "new";

  const [isOpen, setIsOpen] = useState(false);
  const [hasShownTooltip, setHasShownTooltip] = useState(false);

  const tooltipOpacity = useSharedValue(0);
  const tooltipTranslateX = useSharedValue(20);

  useEffect(() => {
    if (isLive && !hasShownTooltip) {
      tooltipOpacity.value = withDelay(
        500,
        withSequence(
          withTiming(1, { duration: 300 }),
          withDelay(10000, withTiming(0, { duration: 300 })),
        ),
      );
      tooltipTranslateX.value = withDelay(
        500,
        withSequence(
          withTiming(0, { duration: 300 }),
          withDelay(10000, withTiming(20, { duration: 300 })),
        ),
      );
      setHasShownTooltip(true);
    }
  }, [isLive, hasShownTooltip]);

  const iconRotate = useAnimatedStyle(() => {
    return {
      transform: [
        {
          rotateZ: withTiming(isOpen ? "240deg" : "0deg", {
            duration: 650,
            easing: Easing.out(Easing.ease),
          }),
        },
      ],
    };
  });

  const tooltipStyle = useAnimatedStyle(() => {
    return {
      opacity: tooltipOpacity.value,
      transform: [{ translateX: tooltipTranslateX.value }],
    };
  });

  return (
    <DropdownMenu onOpenChange={setIsOpen} key={dropdownPortalContainer}>
      <DropdownMenuTrigger>
        <Animated.View style={[iconRotate]}>
          <Cog color={th.theme.colors.foreground} />
        </Animated.View>
        <Animated.View
          style={[
            tooltipStyle,
            {
              position: "absolute",
              right: 30,
              top: 0,
              backgroundColor: "rgba(64,64,64,0.95)",
              borderRadius: 8,
              paddingHorizontal: 8,
              paddingRight: 12,
              paddingVertical: 4,
              flexDirection: "row",
              alignItems: "center",
              gap: 6,
              zIndex: 9999999,
              pointerEvents: "box-none",
              width: 120,
            },
          ]}
        >
          <Text size="sm" color="white">
            End stream here
          </Text>
          <ChevronRight color="white" size={16} style={[zero.mr[4]]} />
        </Animated.View>
      </DropdownMenuTrigger>
      <ResponsiveDropdownMenuContent side="top" align="end">
        {isLive && (
          <DropdownMenuGroup title="Stream">
            <DropdownMenuItem
              closeOnPress={true}
              onPress={() => {
                toggleStopStream();
              }}
            >
              <Text color="destructive">Stop Stream</Text>
            </DropdownMenuItem>
          </DropdownMenuGroup>
        )}
        <DropdownMenuGroup title="Advanced">
          <DropdownMenuCheckboxItem
            checked={debugInfo}
            onCheckedChange={() => setShowDebugInfo(!debugInfo)}
          >
            <Text>Show Debug Info</Text>
          </DropdownMenuCheckboxItem>
        </DropdownMenuGroup>
      </ResponsiveDropdownMenuContent>
    </DropdownMenu>
  );
}
