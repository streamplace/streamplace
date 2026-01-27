import {
  Admonition,
  Button,
  Checkbox,
  Input,
  ResponsiveDialog,
  Text,
  useTheme,
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
      size="sm"
      dismissible={true}
      position="center"
    >
      <View style={[{ maxWidth: 500 }]}>
        <View style={[zero.my[4]]}>
          <Text size="2xl" style={[zero.mb[2]]}>
            Choose your PDS host
          </Text>
          <Text style={[{ color: theme.colors.textMuted }]}>
            Select a Personal Data Server (PDS) to access apps on the
            Atmosphere.
          </Text>
        </View>
        <View style={[zero.pb[2]]}>
          {SHUFFLED_PDS_HOSTS.map((host, index) => (
            <Pressable
              key={host.value}
              onPress={() => handleSelectHost(host.value)}
              style={[
                zero.py[2],
                zero.px[3],
                zero.r.lg,
                {
                  borderWidth: 1,
                  borderColor:
                    !useCustom && selectedHost === host.value
                      ? theme.colors.primary
                      : theme.colors.border,
                  backgroundColor:
                    !useCustom && selectedHost === host.value
                      ? "rgba(0, 122, 255, 0.05)"
                      : "transparent",
                },
                index > 0 && zero.mt[2],
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
              zero.mt[2],
              {
                borderWidth: 1,
                borderColor: useCustom
                  ? theme.colors.primary
                  : theme.colors.border,
                backgroundColor: useCustom
                  ? "rgba(0, 122, 255, 0.05)"
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
                <Text>Custom PDS</Text>
                <Text
                  style={[
                    zero.mt[1],
                    { fontSize: 14, color: theme.colors.textMuted },
                  ]}
                >
                  Enter your own PDS host URL
                </Text>
              </View>
              {useCustom && <Check size={20} color={theme.colors.primary} />}
            </View>
          </Pressable>

          <View style={[zero.mt[4]]}>
            <Pressable
              onPress={handleLearnMore}
              style={[
                zero.layout.flex.row,
                zero.gap.all[1],
                zero.layout.flex.alignCenter,
              ]}
            >
              <Text style={[{ color: theme.colors.ring, fontSize: 14 }]}>
                Learn more about self-hosting
              </Text>
              <ExternalLink size={16} color={theme.colors.ring} />
            </Pressable>
          </View>

          {useCustom && (
            <View style={[zero.mt[3]]}>
              <Text style={[zero.mb[2], { color: theme.colors.textMuted }]}>
                Custom PDS URL
              </Text>
              <Input
                value={customHost}
                onChangeText={setCustomHost}
                placeholder="https://pds.example.com"
                autoCapitalize="none"
                autoCorrect={false}
                keyboardType="url"
              />
            </View>
          )}
          <Admonition variant="info" style={[zero.my[4]] as any}>
            <Text style={[zero.mb[2]]}>
              Each host has their own policies and reliability standards. Your
              ATProto data lives on the host you choose and you can migrate
              later.
            </Text>
            {!useCustom && (
              <Text>
                Read {selectedHostObj?.label}'s{" "}
                <Text
                  onPress={handleTOS}
                  style={[{ color: theme.colors.ring }]}
                >
                  Terms of Service
                </Text>{" "}
                and{" "}
                <Text
                  onPress={handlePrivacy}
                  style={[{ color: theme.colors.ring }]}
                >
                  Privacy Policy
                </Text>{" "}
                before continuing.
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
                zero.my[4],
              ]}
            >
              <Checkbox
                checked={handlePolicyChecked}
                onCheckedChange={hasCheckedHandlePolicy}
              />
              <Text style={[zero.flex[1]]}>
                I have read and agree to the{" "}
                <Text
                  onPress={() =>
                    Linking.openURL(selectedHostObj.handlePolicyDocs!)
                  }
                  style={[{ color: theme.colors.ring }]}
                >
                  {selectedHostObj.label} handle policy
                </Text>
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
            <Text>Cancel</Text>
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
            <Text>Continue</Text>
          </Button>
        </View>
      </View>
    </ResponsiveDialog>
  );
};

export default PdsHostSelectorModal;
