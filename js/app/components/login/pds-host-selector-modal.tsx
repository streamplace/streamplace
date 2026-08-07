import {
  Admonition,
  Button,
  Checkbox,
  Input,
  ResponsiveDialog,
  Trans as T,
  Text,
  useTheme,
  useTranslation,
  zero,
} from "@streamplace/components";
import { Check, ExternalLink } from "lucide-react-native";
import React, { useState } from "react";
import { Linking, Pressable, View } from "react-native";

interface PdsHost {
  value: string;
  label: string;
  description: string;
  handlePolicyDocs?: string;
  terms: string;
  privacy: string;
}

const PDS_HOSTS = [
  {
    value: "https://selfhosted.social",
    label: "selfhosted.social",
    description: "A popular community-run PDS",
    terms: "https://selfhosted.social/legal#terms",
    privacy: "https://selfhosted.social/legal",
  },
  {
    // will redirect to https://bsky.social for sign in :thumb:
    value: "https://witchesbutter.us-west.host.bsky.network",
    label: "Bluesky",
    description: "The main Bluesky PDS instance",
    terms: "https://bsky.social/about/support/tos",
    privacy: "https://bsky.social/about/support/privacy-policy",
  },
  {
    value: "https://blacksky.app",
    label: "Blacksky PDS",
    description: "A PDS service by Blacksky Algorithms",
    terms: "https://blackskyweb.xyz/about/support/tos",
    privacy: "https://blackskyweb.xyz/about/support/privacy-policy/",
    handlePolicyDocs:
      "https://docs.blacksky.community/migrating-to-blacksky-pds-complete-guide#who-can-use-blacksky-services",
  },
  {
    value: "https://pds.tophhie.cloud",
    label: "Tophhie Cloud",
    description: "A PDS service by Tophhie",
    terms: "https://blog.tophhie.cloud/atproto-tos/",
    privacy: "https://blog.tophhie.cloud/atproto-privacy-policy/",
  },
];

// Shuffle the hosts
// items with handle policies should never be first !
const shuffleArray = <T,>(array: T[]): T[] => {
  const arr = [...array];
  for (let i = arr.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [arr[i], arr[j]] = [arr[j], arr[i]];
  }
  return arr;
};

const SHUFFLED_PDS_HOSTS = (() => {
  const withPolicies = PDS_HOSTS.filter((h) => h.handlePolicyDocs);
  const [first, ...withoutPolicies] = PDS_HOSTS.filter(
    (h) => !h.handlePolicyDocs,
  );
  return [first, ...shuffleArray(withPolicies.concat(withoutPolicies))];
})();

interface PdsHostSelectorModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onSubmit: (pdsHost: string) => void;
}

export const PdsHostSelectorModal: React.FC<PdsHostSelectorModalProps> = ({
  open,
  onOpenChange,
  onSubmit,
}) => {
  const [selectedHost, setSelectedHost] = useState<string | null>(
    SHUFFLED_PDS_HOSTS[0].value,
  );
  const [customHost, setCustomHost] = useState<string>("");
  const [useCustom, setUseCustom] = useState(false);
  const [handlePolicyChecked, hasCheckedHandlePolicy] = useState(false);

  const { theme } = useTheme();
  const { t } = useTranslation();

  const selectedHostObj =
    SHUFFLED_PDS_HOSTS.find((host) => host.value === selectedHost) ||
    SHUFFLED_PDS_HOSTS[0];

  const handleCancel = () => {
    setSelectedHost(SHUFFLED_PDS_HOSTS[0].value);
    setCustomHost("");
    setUseCustom(false);
    onOpenChange(false);
  };

  const handleSubmit = () => {
    const hostToUse = useCustom ? customHost : selectedHost;
    if (!hostToUse) return;

    onSubmit(hostToUse);
    handleCancel();
  };

  const handleLearnMore = () => {
    Linking.openURL("https://atproto.com/guides/self-hosting");
  };
  const handleTOS = () => {
    Linking.openURL(selectedHostObj.terms);
  };
  const handlePrivacy = () => {
    Linking.openURL(selectedHostObj.privacy);
  };

  const handleSelectHost = (value: string) => {
    setSelectedHost(value);
    setUseCustom(false);
  };

  const handleSelectCustom = () => {
    setUseCustom(true);
  };

  return (
    <ResponsiveDialog
      open={open}
      onOpenChange={onOpenChange}
      showCloseButton={false}
      variant="default"
      size="md"
      dismissible={false}
      position="center"
    >
      <View style={[{ width: "100%", maxWidth: 640 }]}>
        <View style={[zero.mt[2], zero.mb[3]]}>
          <Text size="2xl" style={[zero.mb[2]]}>
            {t("pds-selector-title")}
          </Text>
          <Text style={[{ color: theme.colors.textMuted }]}>
            {t("pds-selector-description")}
          </Text>
        </View>
        <View style={[zero.pb[2]]}>
          <View
            style={[
              zero.layout.flex.row,
              { flexWrap: "wrap" },
              zero.gap.all[2],
            ]}
          >
            {SHUFFLED_PDS_HOSTS.map((host) => (
              <Pressable
                key={host.value}
                onPress={() => handleSelectHost(host.value)}
                style={[
                  zero.py[2],
                  zero.px[3],
                  zero.r.lg,
                  { flexGrow: 1, flexBasis: "46%", minWidth: 200 },
                  {
                    borderWidth: 1,
                    borderColor:
                      !useCustom && selectedHost === host.value
                        ? theme.colors.primary
                        : theme.colors.border,
                    backgroundColor:
                      !useCustom && selectedHost === host.value
                        ? theme.colors.surface2
                        : "transparent",
                  },
                ]}
              >
                <View
                  style={[
                    zero.layout.flex.row,
                    zero.layout.flex.spaceBetween,
                    zero.layout.flex.alignCenter,
                  ]}
                >
                  <View style={[zero.flex[1]]}>
                    <Text>{host.label}</Text>
                    <Text
                      style={[
                        zero.mt[1],
                        { fontSize: 14, color: theme.colors.textMuted },
                      ]}
                    >
                      {host.description}
                    </Text>
                  </View>
                  {!useCustom && selectedHost === host.value && (
                    <Check size={20} color={theme.colors.primary} />
                  )}
                </View>
              </Pressable>
            ))}

            <Pressable
              onPress={handleSelectCustom}
              style={[
                zero.py[2],
                zero.px[3],
                zero.r.lg,
                { flexGrow: 1, flexBasis: "46%", minWidth: 200 },
                {
                  borderWidth: 1,
                  borderColor: useCustom
                    ? theme.colors.primary
                    : theme.colors.border,
                  backgroundColor: useCustom
                    ? theme.colors.surface2
                    : "transparent",
                },
              ]}
            >
              <View
                style={[
                  zero.layout.flex.row,
                  zero.layout.flex.spaceBetween,
                  zero.layout.flex.alignCenter,
                ]}
              >
                <View style={[zero.flex[1]]}>
                  <Text>{t("pds-selector-custom-label")}</Text>
                  <Text
                    style={[
                      zero.mt[1],
                      { fontSize: 14, color: theme.colors.textMuted },
                    ]}
                  >
                    {t("pds-selector-custom-description")}
                  </Text>
                </View>
                {useCustom && <Check size={20} color={theme.colors.primary} />}
              </View>
            </Pressable>
          </View>

          <View style={[zero.mt[3]]}>
            <Pressable
              onPress={handleLearnMore}
              style={[
                zero.layout.flex.row,
                zero.gap.all[1],
                zero.layout.flex.alignCenter,
              ]}
            >
              <Text style={[{ color: theme.colors.ring, fontSize: 14 }]}>
                {t("pds-selector-learn-more")}
              </Text>
              <ExternalLink size={16} color={theme.colors.ring} />
            </Pressable>
          </View>

          {useCustom && (
            <View style={[zero.mt[3]]}>
              <Text style={[zero.mb[2], { color: theme.colors.textMuted }]}>
                {t("pds-selector-custom-url-label")}
              </Text>
              <Input
                value={customHost}
                onChangeText={setCustomHost}
                placeholder={t("pds-selector-custom-url-placeholder")}
                autoCapitalize="none"
                autoCorrect={false}
                keyboardType="url"
              />
            </View>
          )}
          <Admonition variant="info" style={[zero.my[3]] as any}>
            <Text style={[zero.mb[2]]}>{t("pds-selector-info")}</Text>
            {!useCustom && (
              <Text style={[zero.mb[2]]}>
                <T
                  i18nKey="pds-selector-read-policies"
                  values={{ label: selectedHostObj?.label }}
                  components={{
                    tosLink: (
                      <Text
                        onPress={handleTOS}
                        style={[{ color: theme.colors.ring }]}
                      />
                    ),
                    privacyLink: (
                      <Text
                        onPress={handlePrivacy}
                        style={[{ color: theme.colors.ring }]}
                      />
                    ),
                  }}
                />
              </Text>
            )}
          </Admonition>
          {!useCustom && selectedHostObj.handlePolicyDocs && (
            <View
              style={[
                zero.layout.flex.row,
                zero.layout.flex.align.center,
                zero.layout.flex.justify.start,
                zero.gap.all[2],
                zero.mb[4],
                zero.mt[2],
              ]}
            >
              <Checkbox
                checked={handlePolicyChecked}
                onCheckedChange={hasCheckedHandlePolicy}
              />
              <Text style={[zero.flex[1]]}>
                <T
                  i18nKey="pds-selector-handle-policy-checkbox"
                  components={{
                    policyLink: (
                      <Text
                        onPress={() =>
                          Linking.openURL(selectedHostObj.handlePolicyDocs!)
                        }
                        style={[{ color: theme.colors.ring }]}
                      >
                        {selectedHostObj.label} guidelines and handle policy
                      </Text>
                    ),
                  }}
                />
              </Text>
            </View>
          )}
        </View>
        <View
          style={[
            zero.flex[1],
            zero.layout.flex.row,
            zero.layout.flex.justify.end,
            zero.gap.all[2],
          ]}
        >
          <Button width="min" variant="secondary" onPress={handleCancel}>
            <Text>{t("cancel")}</Text>
          </Button>
          <Button
            width="min"
            variant="primary"
            onPress={handleSubmit}
            disabled={
              (useCustom && !customHost.trim()) ||
              (!handlePolicyChecked && !!selectedHostObj.handlePolicyDocs)
            }
          >
            <Text>{t("continue")}</Text>
          </Button>
        </View>
      </View>
    </ResponsiveDialog>
  );
};

export default PdsHostSelectorModal;
