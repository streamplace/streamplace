import { useIsMobile } from "@/hooks/use-mobile";
import { CONTENT_WARNINGS } from "@/lib/content-warnings";
import { TriangleAlert } from "lucide-react";
import type { RefObject } from "react";
import { useTranslation } from "react-i18next";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "../ui/sheet";

export function ContentWarningOverlay({
  warnings,
  portalContainer,
}: {
  warnings: string[];
  portalContainer?: RefObject<HTMLDivElement | null>;
}) {
  const isMobile = useIsMobile();
  const { t } = useTranslation("common");
  if (warnings.length === 0) return null;

  const warningLabels = warnings.map((warning) => (
    <span
      key={warning}
      className="border-border bg-muted text-foreground rounded-full border px-3 py-1 text-sm font-semibold"
    >
      {CONTENT_WARNINGS.find(
        (item) =>
          item.value === warning ||
          `cwarn:${item.value.split("#").at(-1)}` === warning,
      )?.label ?? warning}
    </span>
  ));

  const trigger = (
    <button
      type="button"
      className="flex items-center gap-2 rounded-md border border-white/20 bg-black/75 px-3 py-2 text-sm font-medium text-white backdrop-blur-sm transition-colors hover:bg-black/85 focus-visible:ring-2 focus-visible:ring-white focus-visible:ring-offset-2 focus-visible:ring-offset-black focus-visible:outline-none"
      onClick={(event) => event.stopPropagation()}
    >
      <TriangleAlert className="size-4" aria-hidden="true" />
      <span>{t("content-warning-badge")}</span>
    </button>
  );

  if (isMobile) {
    return (
      <div className="pointer-events-auto absolute top-3 left-3 z-20">
        <Sheet>
          <SheetTrigger render={trigger} />
          <SheetContent
            side="bottom"
            className="rounded-t-xl pb-[max(1rem,env(safe-area-inset-bottom))]"
            portalContainer={portalContainer}
          >
            <SheetHeader>
              <SheetTitle>{t("content-warning-title")}</SheetTitle>
              <SheetDescription>
                {t("content-warning-description")}
              </SheetDescription>
            </SheetHeader>
            <div className="flex flex-wrap gap-2 px-4 pb-6">
              {warningLabels}
            </div>
          </SheetContent>
        </Sheet>
      </div>
    );
  }

  return (
    <div className="pointer-events-auto absolute top-3 left-3 z-20">
      <DropdownMenu>
        <DropdownMenuTrigger render={trigger} />
        <DropdownMenuContent
          align="start"
          className="w-64"
          portalContainer={portalContainer}
        >
          <div className="px-2 py-1 text-sm font-semibold">
            {t("content-warning-title")}
          </div>
          <p className="text-muted-foreground px-2 pb-2 text-sm">
            {t("content-warning-description")}
          </p>
          <DropdownMenuSeparator className="bg-border" />
          <div className="flex flex-wrap gap-2 p-2">{warningLabels}</div>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
