import { TriggerRef } from "@rn-primitives/dropdown-menu";
import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  DropdownMenu,
  DropdownMenuGroup,
  DropdownMenuInfo,
  DropdownMenuItem,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
} from "../ui/dropdown";
import { Text } from "../ui/text";

function getCategoryKey(category: string): string {
  const categoryMap: Record<string, string> = {
    "place.stream.richtext.defs#discriminatory": "category-discriminatory",
    "place.stream.richtext.defs#sexually_explicit":
      "category-sexually-explicit",
    "place.stream.richtext.defs#profanity": "category-profanity",
  };
  return categoryMap[category] || category;
}

export function CensoredText({
  text,
  reasoning,
}: {
  text: string;
  reasoning?: string[];
}) {
  const { t } = useTranslation("chat");
  const [revealed, setRevealed] = useState(false);
  const dropdownRef = useRef<TriggerRef>(null);
  const handleOpenDropdown = () => {
    dropdownRef.current?.open();
  };

  const translatedReasons = reasoning?.map((r) => t(getCategoryKey(r)));

  return (
    <>
      <Text
        color={revealed ? "default" : "primary"}
        style={{ display: "inline" as any }}
        onPress={handleOpenDropdown}
      >
        {revealed ? text : text.replace(/./g, "*")}
      </Text>
      <DropdownMenu>
        <DropdownMenuTrigger ref={dropdownRef}></DropdownMenuTrigger>
        <ResponsiveDropdownMenuContent>
          <DropdownMenuGroup>
            <DropdownMenuItem onPress={() => setRevealed(!revealed)}>
              <Text>
                {revealed ? t("censored-text-hide") : t("censored-text-reveal")}
              </Text>
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuInfo
            description={
              translatedReasons
                ? t("censored-text-blocked-with-reasons", {
                    reasons: translatedReasons.join(", "),
                  })
                : t("censored-text-blocked-unknown")
            }
          />
        </ResponsiveDropdownMenuContent>
      </DropdownMenu>
    </>
  );
}
