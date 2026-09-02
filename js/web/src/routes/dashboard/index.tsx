import { ControlPanel } from "@/components/dashboard/control-panel";
import {
  useDashboardStore,
  useIsDashboardStoreReady,
} from "@/components/dashboard/dashboard-store-context";
import { useSession } from "@/lib/session";
import { createFileRoute } from "@tanstack/react-router";
import { Loader } from "lucide-react";

export const Route = createFileRoute("/dashboard/")({
  component: DashboardIndex,
});

function DashboardIndex() {
  const { state } = useSession();
  if (state.status !== "authenticated") {
    return null;
  }
  // There are situations in which this DashboardIndex loads
  // BEFORE the LivestreamStore is ready (e.g. in-app navigation)
  // so we'll need to handle those properly by waiting for the
  // store to be ready before rendering the dashboard content.
  const isReady = useIsDashboardStoreReady();

  if (!isReady) {
    return <DashboardLoadingSkeleton />;
  }

  return <DashboardInner />;
}

function DashboardInner() {
  const { state } = useSession();
  const store = useDashboardStore();

  if (state.status !== "authenticated") {
    return null;
  }

  const user = state.session.did;

  return <ControlPanel store={store} user={user} />;
}

function DashboardLoadingSkeleton() {
  return (
    <div className="flex h-full items-center justify-center">
      <Loader className="animate-spin" />
    </div>
  );
}
