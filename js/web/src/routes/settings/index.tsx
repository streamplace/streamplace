import { createFileRoute, Navigate } from "@tanstack/react-router";

export const Route = createFileRoute("/settings/")({
  component: () => <Navigate to="/settings/account" />,
});
