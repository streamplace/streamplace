import {
  AppBskyActorDefs,
  ComAtprotoModerationCreateReport,
} from "@atproto/api";
import { useRootContext } from "@rn-primitives/dropdown-menu";
import {
  DropdownMenu,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
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
import { LivestreamViewHydrated } from "streamplace";

type ReportSubject =
  | ComAtprotoModerationCreateReport.InputSchema["subject"]
  | null;

interface KebabMenuProps {
  dropdownPortalContainer?: string;
}

export function KebabMenu({ dropdownPortalContainer }: KebabMenuProps) {
  const th = useTheme();
  const [isOpen, setIsOpen] = useState(false);
  const [showUpdateTitleDialog, setShowUpdateTitleDialog] = useState(false);

  const livestreamFromStore = useLivestreamStore((x) => x.livestream);
  const livestream = useLivestream();
  const setReportModalOpen = usePlayerStore((x) => x.setReportModalOpen);
  const setReportSubject = usePlayerStore((x) => x.setReportSubject);
  const { profile } = useLivestreamInfo();

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
          <DropdownMenuGroup title="Report">
            <ReportStreamItem
              livestream={livestreamFromStore}
              setReportModalOpen={setReportModalOpen}
              setReportSubject={setReportSubject}
            />
            <ReportBioPageItem
              profile={profile}
              setReportModalOpen={setReportModalOpen}
              setReportSubject={setReportSubject}
            />
            <ReportUserItem
              profile={profile}
              setReportModalOpen={setReportModalOpen}
              setReportSubject={setReportSubject}
            />
          </DropdownMenuGroup>
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

function ReportStreamItem({
  livestream,
  setReportModalOpen,
  setReportSubject,
}: {
  livestream: LivestreamViewHydrated | null;
  setReportModalOpen: (open: boolean) => void;
  setReportSubject: (subject: ReportSubject) => void;
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
      disabled={!livestream}
    >
      <Text>Report Livestream...</Text>
    </DropdownMenuItem>
  );
}

function ReportBioPageItem({
  profile,
  setReportModalOpen,
  setReportSubject,
}: {
  profile: AppBskyActorDefs.ProfileViewBasic | null;
  setReportModalOpen: (open: boolean) => void;
  setReportSubject: (subject: ReportSubject) => void;
}) {
  const { onOpenChange } = useRootContext();

  return (
    <DropdownMenuItem
      onPress={() => {
        if (!profile?.did) return;
        onOpenChange?.(false);
        setReportModalOpen(true);
        setReportSubject({
          $type: "com.atproto.admin.defs#repoRef",

          did: profile.did,
        });
      }}
      disabled={!profile?.did}
    >
      <Text>Report Bio Page...</Text>
    </DropdownMenuItem>
  );
}

function ReportUserItem({
  profile,
  setReportModalOpen,
  setReportSubject,
}: {
  profile: AppBskyActorDefs.ProfileViewBasic | null;
  setReportModalOpen: (open: boolean) => void;
  setReportSubject: (subject: ReportSubject) => void;
}) {
  const { onOpenChange } = useRootContext();

  return (
    <DropdownMenuItem
      onPress={() => {
        if (!profile?.did) return;
        onOpenChange?.(false);
        setReportModalOpen(true);
        setReportSubject({
          $type: "com.atproto.admin.defs#repoRef",
          did: profile.did,
        });
      }}
      disabled={!profile?.did}
    >
      <Text>Report User...</Text>
    </DropdownMenuItem>
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
