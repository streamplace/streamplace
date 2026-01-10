import React, { useEffect, useState } from "react";
import { createPortal } from "react-dom";

function Portal({
  children,
  hostName = "INTERNAL_PRIMITIVE_DEFAULT_HOST_NAME",
}: {
  children: React.ReactNode;
  hostName?: string;
}) {
  const [hostElement, setHostElement] = useState<HTMLElement | null>(null);

  useEffect(() => {
    const element = document.querySelector<HTMLElement>(
      `[data-portal-host="${hostName}"]`,
    );
    setHostElement(element);
  }, [hostName]);

  if (!hostElement) {
    return null;
  }

  return createPortal(children, hostElement);
}

interface PortalHostProps {
  name?: string;
}

function PortalHost({
  name = "INTERNAL_PRIMITIVE_DEFAULT_HOST_NAME",
}: PortalHostProps) {
  return <div data-portal-host={name} />;
}

export { Portal, PortalHost };
