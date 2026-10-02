import { CONTENT_WARNINGS } from "@/lib/content-warnings";
import { TriangleAlert } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";

export function ContentWarningOverlay({
  warnings,
}: {
  warnings: string[];
}) {
  if (warnings.length === 0) return null;

  return (
    <div className="pointer-events-auto absolute top-3 left-3 z-20">
      <DropdownMenu>
        <DropdownMenuTrigger
          className="flex items-center gap-2 rounded-md border border-white/20 bg-black/75 px-3 py-2 text-sm font-medium text-white backdrop-blur-sm transition-colors hover:bg-black/85 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white focus-visible:ring-offset-2 focus-visible:ring-offset-black"
          aria-label="Content warnings"
          onClick={(event) => event.stopPropagation()}
        >
          <TriangleAlert className="size-4" aria-hidden="true" />
          <span>Intended for certain audiences</span>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          align="start"
          className="w-64 border-white/20 bg-black/90 text-white backdrop-blur-sm"
        >
          <DropdownMenuLabel className="px-2 py-1 text-sm font-semibold text-white">
            Heads up!
          </DropdownMenuLabel>
          <p className="px-2 pb-2 text-sm text-white/70">
            This video may contain:
          </p>
          <DropdownMenuSeparator className="bg-white/15" />
          <div className="flex flex-wrap gap-2 p-2">
            {warnings.map((warning) => (
              <span
                key={warning}
                className="rounded-full border border-white/20 bg-white/10 px-3 py-1 text-sm font-semibold text-white"
              >
                {CONTENT_WARNINGS.find((item) => item.value === warning)
                  ?.label ?? warning}
              </span>
            ))}
          </div>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
