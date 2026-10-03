import { captionLanguageName } from "@streamplace/core";
import { Link } from "@tanstack/react-router";
import { Captions, CaptionsOff, ChevronDown } from "lucide-react";
import { useTranslation } from "react-i18next";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "../ui/dropdown-menu";
import type { PlayerCaptions } from "./use-player-captions";

export function CaptionsButton({
  captions,
  onOpenChange,
}: {
  captions: PlayerCaptions;
  onOpenChange?: (open: boolean) => void;
}) {
  const { t, i18n } = useTranslation();
  const Icon = captions.enabled ? Captions : CaptionsOff;
  return (
    <div
      className="flex items-center"
      onClick={(event) => event.stopPropagation()}
    >
      <button
        type="button"
        data-testid="player-cc-button"
        aria-label={t("player-captions")}
        aria-pressed={captions.enabled}
        onClick={captions.toggle}
        className="focus-visible:ring-ring text-primary-foreground hover:bg-muted flex size-11 items-center justify-center rounded-md focus-visible:ring-2"
      >
        <Icon className="size-5" />
      </button>
      <DropdownMenu onOpenChange={onOpenChange}>
        <DropdownMenuTrigger
          data-testid="player-cc-menu-button"
          aria-label={t("player-captions-menu")}
          className="focus-visible:ring-ring text-primary-foreground hover:bg-muted flex size-11 items-center justify-center rounded-md focus-visible:ring-2"
        >
          <ChevronDown className="size-4" />
        </DropdownMenuTrigger>
        <DropdownMenuContent
          side="top"
          align="end"
          data-testid="player-cc-menu"
        >
          <DropdownMenuRadioGroup
            value={captions.enabled ? (captions.track?.id ?? "on") : "off"}
            onValueChange={(id) =>
              captions.select(
                captions.tracks.find((track) => track.id === id) ?? null,
              )
            }
          >
            <DropdownMenuRadioItem
              value="off"
              data-testid="player-cc-track-off"
            >
              {t("player-captions-off")}
            </DropdownMenuRadioItem>
            {captions.tracks.map((track) => (
              <DropdownMenuRadioItem
                key={track.id}
                value={track.id}
                data-testid={`player-cc-track-${track.id}`}
              >
                {track.source === "auto"
                  ? t("player-captions-track-auto", {
                      language: captionLanguageName(
                        track.language,
                        i18n.language,
                      ),
                    })
                  : captionLanguageName(track.language, i18n.language)}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
          <Link
            to="/settings/captions"
            data-testid="player-cc-settings"
            className="focus-visible:ring-ring hover:bg-muted block rounded-md px-3 py-2 text-sm focus-visible:ring-2"
          >
            {t("player-captions-style")}
          </Link>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
