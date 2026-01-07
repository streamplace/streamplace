import {
  DropdownMenu,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Input,
  manifest,
  MenuContainer,
  MenuGroup,
  ResponsiveDropdownMenuContent,
  Text,
  useTheme,
  View,
  zero,
} from "@streamplace/components";
import { Check, ChevronDown, Handshake, HardHat } from "lucide-react-native";
import React, { useState } from "react";
import { useTranslation } from "react-i18next";
import { ScrollView } from "react-native";

export function LanguagesCategorySettings() {
  const [languageSearchQuery, setLanguageSearchQuery] = useState("");
  const { t, i18n } = useTranslation("settings");
  const { theme } = useTheme();

  const filteredLanguages = Object.entries(manifest.languages).filter(
    ([code, info]) =>
      languageSearchQuery === "" ||
      info.name.toLowerCase().includes(languageSearchQuery.toLowerCase()) ||
      info.nativeName
        .toLowerCase()
        .includes(languageSearchQuery.toLowerCase()) ||
      code.toLowerCase().includes(languageSearchQuery.toLowerCase()),
  );

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <MenuContainer>
            <View>
              <Text size="xl" style={{ marginBottom: 4 }}>
                {t("language-selection")}
              </Text>
              <Text size="lg" color="muted" style={{ marginBottom: 8 }}>
                {t("language-selection-description")}
              </Text>
              <MenuGroup>
                <DropdownMenu>
                  <DropdownMenuTrigger>
                    <View
                      style={{
                        width: "100%",
                        paddingVertical: 12,
                        paddingHorizontal: 8,
                      }}
                    >
                      <View
                        style={{
                          flexDirection: "row",
                          alignItems: "center",
                          gap: 8,
                          justifyContent: "space-between",
                          width: "100%",
                        }}
                      >
                        <Text>
                          {manifest.languages[
                            i18n.language as keyof typeof manifest.languages
                          ]?.flag || "🌍"}{" "}
                          {manifest.languages[
                            i18n.language as keyof typeof manifest.languages
                          ]?.nativeName || i18n.language}
                        </Text>
                        <ChevronDown
                          size={18}
                          color={theme.colors.mutedForeground}
                        />
                      </View>
                    </View>
                  </DropdownMenuTrigger>

                  <ResponsiveDropdownMenuContent>
                    <ScrollView
                      stickyHeaderIndices={[0]}
                      showsVerticalScrollIndicator={false}
                      style={{ maxHeight: "60vh" } as any}
                    >
                      <View
                        style={{
                          paddingVertical: 8,
                        }}
                      >
                        <Input
                          placeholder={t("input-search-languages")}
                          variant="filled"
                          value={languageSearchQuery}
                          onChangeText={setLanguageSearchQuery}
                          size="sm"
                        />
                      </View>

                      <DropdownMenuGroup>
                        {filteredLanguages.map(([code, info], i) => (
                          <React.Fragment key={code}>
                            <DropdownMenuItem
                              onPress={() => {
                                i18n.changeLanguage(code);
                                setLanguageSearchQuery("");
                              }}
                            >
                              <View
                                style={{
                                  flexDirection: "row",
                                  alignItems: "center",
                                  justifyContent: "space-between",
                                  width: "100%",
                                }}
                              >
                                <View
                                  style={{
                                    flexDirection: "row",
                                    alignItems: "center",
                                    gap: 8,
                                  }}
                                >
                                  <Text>{info.flag}</Text>
                                  <View
                                    style={{
                                      paddingVertical:
                                        info.name === info.nativeName ? 6 : 0,
                                    }}
                                  >
                                    <Text
                                      style={{
                                        fontWeight:
                                          i18n.language === code
                                            ? "bold"
                                            : "normal",
                                        lineHeight: 18,
                                      }}
                                    >
                                      {info.nativeName}
                                    </Text>
                                    {info.name !== info.nativeName && (
                                      <Text
                                        style={{
                                          fontSize: 12,
                                          opacity: 0.7,
                                          lineHeight: 18,
                                        }}
                                      >
                                        {info.name}
                                      </Text>
                                    )}
                                  </View>
                                </View>
                                {i18n.language === code && (
                                  <Text
                                    style={{
                                      color: "white",
                                      fontWeight: "bold",
                                    }}
                                  >
                                    <Check />
                                  </Text>
                                )}
                              </View>
                            </DropdownMenuItem>
                            {i < filteredLanguages.length - 1 && (
                              <DropdownMenuSeparator />
                            )}
                          </React.Fragment>
                        ))}

                        {filteredLanguages.length === 0 && (
                          <View style={{ padding: 12 }}>
                            <Text style={{ opacity: 0.7, textAlign: "center" }}>
                              {t("no-languages-found")}
                            </Text>
                          </View>
                        )}
                      </DropdownMenuGroup>
                    </ScrollView>
                  </ResponsiveDropdownMenuContent>
                </DropdownMenu>
              </MenuGroup>
            </View>

            <View style={[zero.gap.all[1]]}>
              <View
                style={[
                  zero.layout.flex.row,
                  zero.gap.all[2],
                  zero.layout.flex.alignCenter,
                ]}
              >
                <HardHat size={24} color={theme.colors.mutedForeground} />
                <Text size="xl">{t("currently-translating")}</Text>
              </View>
              <Text size="lg" color="muted">
                {t("currently-translating-description")}
              </Text>
            </View>

            <View style={[zero.gap.all[1]]}>
              <View
                style={[
                  zero.layout.flex.row,
                  zero.gap.all[2],
                  zero.layout.flex.alignCenter,
                ]}
              >
                <Handshake size={24} color="#999" />
                <Text size="xl">{t("help-translate")}</Text>
              </View>
              <Text size="lg" color="muted">
                {t("help-translate-description")}
              </Text>
            </View>
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}
