import { createFileRoute, Outlet } from "@tanstack/react-router";

export const Route = createFileRoute("/settings")({
  component: SettingsLayout,
});

function SettingsLayout() {
  return (
    <div className="w-full max-w-md self-center px-6 py-10">
      <Outlet />
    </div>
  );
}
