import { useRootContext } from "@rn-primitives/dropdown-menu";
import {
  DropdownMenu,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
  PlayerUI,
  ResponsiveDropdownMenuContent,
  Text,
  UpdateStreamTitleDialog,
  useCanModerate,
  useLivestream,
  useLivestreamInfo,
  useLivestreamStore,
  usePlayerStore,
  useTheme,
  useUpdateLivestreamRecord,
} from "@streamplace/components";
import { EllipsisVertical } from "lucide-react-native";
import { useState } from "react";
import Animated, {
  Easing,
  useAnimatedStyle,
  withTiming,
} from "react-native-reanimated";

interface KebabMenuProps {
  dropdownPortalContainer?: string;
}

export function KebabMenu({ dropdownPortalContainer }: KebabMenuProps) {
  const th = useTheme();
  const [isOpen, setIsOpen] = useState(false);
  const [showUpdateTitleDialog, setShowUpdateTitleDialog] = useState(false);

  const livestream = useLivestream();
  const livestreamFromStore = useLivestreamStore((x) => x.livestream);
  const { profile } = useLivestreamInfo();
  const setReportModalOpen = usePlayerStore((x) => x.setReportModalOpen);
  const setReportSubject = usePlayerStore((x) => x.setReportSubject);

  // Get the streamer's DID from the profile
  const streamerDID = profile?.did;
  // Check moderation permissions for the current user on this streamer's channel
  const modPermissions = useCanModerate(streamerDID);
  const { updateLivestream, isLoading: isUpdateTitleLoading } =
    useUpdateLivestreamRecord();

  const iconRotate = useAnimatedStyle(() => {
    return {
      transform: [
        {
          rotateZ: withTiming(isOpen ? "5deg" : "0deg", {
            duration: 200,
            easing: Easing.out(Easing.ease),
          }),
        },
      ],
    };
  });

  return (
    <>
      <DropdownMenu onOpenChange={setIsOpen} key={dropdownPortalContainer}>
        <DropdownMenuTrigger>
          <Animated.View style={[iconRotate]}>
            <EllipsisVertical color={th.theme.colors.foreground} />
          </Animated.View>
        </DropdownMenuTrigger>
        <ResponsiveDropdownMenuContent
          side="top"
          align="end"
          portalHost={dropdownPortalContainer}
        >
          {modPermissions.canManageLivestream && (
            <DropdownMenuGroup title="Stream Settings">
              <UpdateStreamTitleItem
                setShowUpdateTitleDialog={setShowUpdateTitleDialog}
                isUpdateTitleLoading={isUpdateTitleLoading}
                livestream={livestream}
              />
            </DropdownMenuGroup>
          )}
          <PlayerUI.ReportMenuItems
            livestream={livestreamFromStore}
            profile={profile}
            setReportModalOpen={setReportModalOpen}
            setReportSubject={setReportSubject}
          />
        </ResponsiveDropdownMenuContent>
      </DropdownMenu>

      {showUpdateTitleDialog && (
        <UpdateStreamTitleDialog
          livestream={livestream}
          streamerDID={streamerDID}
          updateLivestream={updateLivestream}
          isLoading={isUpdateTitleLoading}
          onClose={() => setShowUpdateTitleDialog(false)}
        />
      )}
    </>
  );
}

function UpdateStreamTitleItem({
  setShowUpdateTitleDialog,
  isUpdateTitleLoading,
  livestream,
}: {
  setShowUpdateTitleDialog: (show: boolean) => void;
  isUpdateTitleLoading: boolean;
  livestream: any;
}) {
  const { onOpenChange } = useRootContext();

  return (
    <DropdownMenuItem
      onPress={() => {
        onOpenChange?.(false);
        setShowUpdateTitleDialog(true);
      }}
      disabled={isUpdateTitleLoading || !livestream}
    >
      <Text>
        {isUpdateTitleLoading ? "Updating..." : "Update stream title"}
      </Text>
    </DropdownMenuItem>
  );
}
