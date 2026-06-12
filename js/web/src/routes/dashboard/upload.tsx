import { UploadForm } from "@/components/dashboard/upload-form";
import { useUpload } from "@/hooks/use-upload";
import { useIsReady, useUserProfile } from "@/lib/store/hooks";
import { createFileRoute } from "@tanstack/react-router";
import { LoaderCircle } from "lucide-react";

export const Route = createFileRoute("/dashboard/upload")({
  component: DashboardUploadPage,
});

function DashboardUploadPage() {
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const upload = useUpload();

  if (!isReady) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <LoaderCircle className="animate-spin text-(--color-fg-muted)" />
      </div>
    );
  }
  if (!userProfile) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center px-6">
        <p className="text-sm text-(--color-fg-muted)">
          Please log in to upload videos.
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-240 px-4 py-4">
      <UploadForm upload={upload} />
    </div>
  );
}
