import { cn } from "@/lib/utils";
import { useEffect, useState } from "react";

export function StreamAvatar({
  avatar,
  label,
  className,
}: {
  avatar?: string;
  label: string;
  className?: string;
}) {
  const [imageFailed, setImageFailed] = useState(false);

  useEffect(() => {
    setImageFailed(false);
  }, [avatar]);

  const initial = label.trim().replace(/^@/, "").charAt(0).toUpperCase() || "?";

  return (
    <div
      aria-hidden="true"
      className={cn(
        "relative flex shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--color-border) bg-(--color-bg-elevated) text-sm font-semibold text-(--color-accent)",
        className,
      )}
    >
      <span>{initial}</span>
      {avatar && !imageFailed && (
        <img
          src={avatar}
          alt=""
          className="absolute inset-0 h-full w-full object-cover"
          onError={() => setImageFailed(true)}
        />
      )}
    </div>
  );
}
