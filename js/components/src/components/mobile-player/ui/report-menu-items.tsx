import {
  AppBskyActorDefs,
  ComAtprotoModerationCreateReport,
} from "@atproto/api";
import { useRootContext } from "@rn-primitives/dropdown-menu";
import { LivestreamViewHydrated } from "streamplace";
import { DropdownMenuGroup, DropdownMenuItem, Text } from "../../ui";

type ReportSubject =
  | ComAtprotoModerationCreateReport.InputSchema["subject"]
  | null;

export interface ReportMenuItemsProps {
  livestream: LivestreamViewHydrated | null;
  profile: AppBskyActorDefs.ProfileViewBasic | null;
  setReportModalOpen: (open: boolean) => void;
  setReportSubject: (subject: ReportSubject) => void;
}

export function ReportMenuItems({
  livestream,
  profile,
  setReportModalOpen,
  setReportSubject,
}: ReportMenuItemsProps) {
  return (
    <DropdownMenuGroup title="Report">
      <ReportStreamItem
        livestream={livestream}
        setReportModalOpen={setReportModalOpen}
        setReportSubject={setReportSubject}
      />
      <ReportUserItem
        profile={profile}
        setReportModalOpen={setReportModalOpen}
        setReportSubject={setReportSubject}
      />
    </DropdownMenuGroup>
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
