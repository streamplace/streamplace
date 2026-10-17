import { ShieldCheck, Trash2 } from "lucide-react-native";
import { useState } from "react";
import { Pressable, Text, View } from "react-native";
import { Button, Input, SegmentedTabs, useToast } from "../../components/ui";
import { atoms } from "../../lib/theme";
import { useTheme } from "../../lib/theme/theme";
import {
  ChatAccessRuleRecord,
  useAddChatAccessRule,
  useListChatAccessRules,
  useRemoveChatAccessRule,
} from "../../streamplace-store/chat-access";

const { flex, r, borders, p, layout, gap, mb } = atoms;

interface ChatAccessPanelProps {
  embedded?: boolean;
}

function describeSubject(rule: ChatAccessRuleRecord): string {
  const s: any = rule.value.subject;
  if (s?.$type === "place.stream.chat.access#verifier") {
    return `accounts verified by ${s.did}`;
  }
  if (s?.$type === "place.stream.chat.access#label") {
    return `accounts labelled ${s.value} by ${s.labeler}`;
  }
  return `a subject this app doesn't know (${s?.$type ?? "?"})`;
}

/**
 * The streamer's chat access rules (place.stream.chat.access records): who
 * may chat on their streams. With no rules everyone may; a deny rule always
 * refuses its matches; once any allow rule exists, only accounts matching an
 * allow rule may chat, and those wear the verified badge in chat.
 */
export default function ChatAccessPanel({
  embedded = false,
}: ChatAccessPanelProps) {
  const { theme } = useTheme();
  const toast = useToast();
  const { rules, isLoading, error, refresh } = useListChatAccessRules();
  const { addRule, isLoading: adding } = useAddChatAccessRule();
  const { removeRule, isLoading: removing } = useRemoveChatAccessRule();

  const [action, setAction] = useState<"allow" | "deny">("allow");
  const [subjectType, setSubjectType] = useState<"verifier" | "label">(
    "verifier",
  );
  const [verifier, setVerifier] = useState("");
  const [labeler, setLabeler] = useState("");
  const [labelValue, setLabelValue] = useState("");

  const canAdd =
    !adding &&
    (subjectType === "verifier"
      ? verifier.trim() !== ""
      : labeler.trim() !== "" && labelValue.trim() !== "");

  const handleAdd = async () => {
    try {
      await addRule(
        action,
        subjectType === "verifier"
          ? { type: "verifier", did: verifier }
          : { type: "label", labeler, value: labelValue },
      );
      setVerifier("");
      setLabeler("");
      setLabelValue("");
      toast.show("Rule added", undefined, { duration: 3, variant: "success" });
      refresh();
    } catch (e) {
      toast.show(
        "Could not add the rule",
        e instanceof Error ? e.message : undefined,
        { duration: 5, variant: "error" },
      );
    }
  };

  const handleRemove = async (rule: ChatAccessRuleRecord) => {
    try {
      await removeRule(rule.rkey);
      toast.show("Rule removed", undefined, {
        duration: 3,
        variant: "success",
      });
      refresh();
    } catch (e) {
      toast.show(
        "Could not remove the rule",
        e instanceof Error ? e.message : undefined,
        { duration: 5, variant: "error" },
      );
    }
  };

  const containerStyle = embedded
    ? [layout.flex.column]
    : [
        { backgroundColor: theme.colors.surface1 },
        r.lg,
        borders.width.thin,
        { borderColor: theme.colors.borderStrong },
        layout.flex.column,
      ];
  const label = { color: theme.colors.text2, fontSize: 13, marginBottom: 6 };
  const hasAllow = rules.some((x) => x.value.action === "allow");

  return (
    <View style={containerStyle}>
      <View
        style={[
          layout.flex.row,
          layout.flex.alignCenter,
          gap.all[2],
          borders.bottom.width.thin,
          { borderBottomColor: theme.colors.borderStrong },
          p[4],
        ]}
      >
        <ShieldCheck size={18} color={theme.colors.text1} />
        <Text
          style={{ color: theme.colors.text1, fontSize: 18, fontWeight: "600" }}
        >
          Chat access
        </Text>
      </View>

      <View style={[p[4], gap.all[4]]}>
        <Text style={{ color: theme.colors.text2, fontSize: 13 }}>
          {rules.length === 0
            ? "Anyone can chat. Add an allow rule to restrict chat to accounts a verifier or labeler vouches for; those accounts wear a verified badge in your chat."
            : hasAllow
              ? "Only accounts matching an allow rule can chat; deny rules always win."
              : "Anyone can chat except accounts matching a deny rule."}
        </Text>

        {error ? (
          <Text style={{ color: theme.colors.danger, fontSize: 13 }}>
            {error}
          </Text>
        ) : null}

        <View style={[gap.all[2]]}>
          {rules.map((rule) => (
            <View
              key={rule.uri}
              style={[
                layout.flex.row,
                layout.flex.alignCenter,
                layout.flex.spaceBetween,
                gap.all[3],
                p[3],
                r.md,
                { backgroundColor: theme.colors.surface2 },
              ]}
            >
              <View style={[flex.values[1]]}>
                <Text
                  style={{
                    color:
                      rule.value.action === "deny"
                        ? theme.colors.danger
                        : theme.colors.success,
                    fontSize: 12,
                    fontWeight: "600",
                    textTransform: "uppercase",
                  }}
                >
                  {rule.value.action}
                </Text>
                <Text style={{ color: theme.colors.text1, fontSize: 13 }}>
                  {describeSubject(rule)}
                </Text>
              </View>
              <Pressable
                onPress={() => handleRemove(rule)}
                disabled={removing}
                accessibilityLabel="Remove rule"
                style={[p[2]]}
              >
                <Trash2 size={16} color={theme.colors.text2} />
              </Pressable>
            </View>
          ))}
          {isLoading && rules.length === 0 ? (
            <Text style={{ color: theme.colors.text3, fontSize: 13 }}>
              Loading…
            </Text>
          ) : null}
        </View>

        <View
          style={[
            gap.all[3],
            p[3],
            r.md,
            borders.width.thin,
            { borderColor: theme.colors.border },
          ]}
        >
          <Text style={label}>New rule</Text>
          <SegmentedTabs
            size="sm"
            options={[
              { value: "allow", label: "Allow" },
              { value: "deny", label: "Deny" },
            ]}
            value={action}
            onChange={(v) => setAction(v as "allow" | "deny")}
          />
          <SegmentedTabs
            size="sm"
            options={[
              { value: "verifier", label: "Verified by" },
              { value: "label", label: "Labelled by" },
            ]}
            value={subjectType}
            onChange={(v) => setSubjectType(v as "verifier" | "label")}
          />
          {subjectType === "verifier" ? (
            <View style={[mb[1]]}>
              <Text style={label}>Verifier DID or handle</Text>
              <Input
                value={verifier}
                onChangeText={setVerifier}
                placeholder="did:plc:... or @verifier.example"
                onSubmitEditing={() => canAdd && handleAdd()}
                returnKeyType="done"
              />
            </View>
          ) : (
            <View style={[gap.all[2]]}>
              <View>
                <Text style={label}>Labeler DID or handle</Text>
                <Input
                  value={labeler}
                  onChangeText={setLabeler}
                  placeholder="did:plc:... or @labeler.example"
                />
              </View>
              <View>
                <Text style={label}>Label (trailing * matches a prefix)</Text>
                <Input
                  value={labelValue}
                  onChangeText={setLabelValue}
                  placeholder="verified"
                  onSubmitEditing={() => canAdd && handleAdd()}
                  returnKeyType="done"
                />
              </View>
            </View>
          )}
          <Button onPress={handleAdd} disabled={!canAdd} width="min">
            Add rule
          </Button>
        </View>
      </View>
    </View>
  );
}
