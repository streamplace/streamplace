import {
  forwardRef,
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";
import {
  Linking,
  Platform,
  Pressable,
  ScrollView,
  useWindowDimensions,
  View,
} from "react-native";
import {
  CONTENT_WARNINGS,
  LICENSE_OPTIONS,
} from "../../lib/metadata-constants";
import { breakpoints } from "../../lib/theme/tokens";

import { ExternalLink } from "lucide-react-native";
import { place } from "streamplace";
import {
  useGetBroadcasterDID,
  useGetContentMetadata,
  useSaveContentMetadata,
} from "../../streamplace-store/content-metadata-actions";
import {
  useDID,
  useStreamplaceStore,
} from "../../streamplace-store/streamplace-store";
import { usePDSAgent } from "../../streamplace-store/xrpc";
import * as zero from "../../ui";
import { Admonition, SegmentedTabs } from "../ui";
import { Button } from "../ui/button";
import { Checkbox } from "../ui/checkbox";
import { ExplanationDialog } from "../ui/explanation-dialog";
import { Input } from "../ui/input";
import { Select } from "../ui/select";
import { Text } from "../ui/text";
import { Textarea } from "../ui/textarea";
import { useToast } from "../ui/toast";
import { Tooltip } from "../ui/tooltip";

const { p, r, bg, borders, w, text, layout, gap, flex } = zero;
const distributionExplanations = [
  {
    label: "Allow everyone to distribute your content",
    description:
      "Anyone can distribute your content, but they still have to respect the retention setting below.",
  },
  {
    label: "Mark your content as available for generative AI training",
    description:
      "Your video's metadata signals whether you want your content used to train generative AI.",
  },
  {
    label: "Allow everyone to archive your content",
    description: "Anyone may archive your content indefinitely.",
  },
];

function mergeEditedFields<T extends object>(
  base: T | undefined,
  draft: T,
  edited: ReadonlyMap<string, number>,
): T {
  const result = { ...base } as T;
  for (const field of edited.keys()) {
    const key = field as keyof T;
    result[key] = draft[key];
  }
  return result;
}

function clearSavedFields(
  pending: Map<string, number>,
  saved: ReadonlyMap<string, number>,
) {
  for (const [field, revision] of saved) {
    if (pending.get(field) === revision) pending.delete(field);
  }
}

function mergeWarningChoices(
  base: string[] | undefined,
  draft: string[],
  edited: ReadonlyMap<string, number>,
): string[] {
  const warnings =
    base?.filter(
      (warning) => !edited.has(warning) || draft.includes(warning),
    ) ?? [];
  for (const warning of edited.keys()) {
    if (draft.includes(warning) && !warnings.includes(warning))
      warnings.push(warning);
  }
  return warnings;
}

export interface ContentMetadataFormProps {
  showUpdateButton?: boolean;
  onMetadataChange?: (
    metadata: place.stream.metadata.configuration.Main,
  ) => void;
  initialMetadata?: place.stream.metadata.configuration.Main;
  scrollable?: boolean;
  warningsOnly?: boolean;
  showAllSections?: boolean;
  style?: any;
}

export const ContentMetadataForm = forwardRef<any, ContentMetadataFormProps>(
  (
    {
      showUpdateButton = false,
      onMetadataChange,
      initialMetadata,
      scrollable = true,
      warningsOnly = false,
      showAllSections = false,
      style,
    },
    ref,
  ) => {
    const pdsAgent = usePDSAgent();
    const did = useDID();
    const getContentMetadata = useGetContentMetadata();
    const saveContentMetadata = useSaveContentMetadata();
    const toast = useToast();
    const th = zero.useTheme();
    const { width } = useWindowDimensions();
    const fieldIds = useId();
    const showWarningDescriptions =
      Platform.OS === "web" && width >= breakpoints.md;
    const allSections = showAllSections && !warningsOnly;
    const preserveWarnings =
      Platform.OS === "web" && (allSections || warningsOnly);
    const stackedFields = allSections && width < breakpoints.md;
    const stackedFieldStyle = stackedFields
      ? { flexDirection: "column" as const, alignItems: "stretch" as const }
      : undefined;
    const fieldLabelStyle = allSections
      ? {
          minWidth: stackedFields ? th.theme.spacing[0] : th.theme.spacing[32],
          maxWidth: stackedFields ? ("100%" as const) : th.theme.spacing[32],
          paddingBottom: stackedFields
            ? th.theme.spacing[1]
            : th.theme.spacing[0],
          paddingRight: stackedFields
            ? th.theme.spacing[0]
            : th.theme.spacing[2],
        }
      : undefined;
    const Container = scrollable ? ScrollView : View;

    // Local state for metadata
    const [contentWarnings, setContentWarnings] = useState<string[]>([]);
    const [distributionPolicy, setDistributionPolicy] =
      useState<place.stream.metadata.distributionPolicy.Main>({});
    const [contentRights, setContentRights] =
      useState<place.stream.metadata.contentRights.Main>({});
    const [selectedLicense, setSelectedLicense] = useState<string>("");
    const [customLicenseText, setCustomLicenseText] = useState<string>("");
    const [loading, setLoading] = useState(false);
    const [loadingDistribution, setLoadingDistribution] = useState(
      allSections && !initialMetadata,
    );
    const [distributionLoadError, setDistributionLoadError] = useState(false);
    const [metadataRetry, setMetadataRetry] = useState(0);
    const [hasMetadata, setHasMetadata] = useState(false);
    const editedSections = useRef(new Set<string>());
    const editedWarnings = useRef(new Map<string, number>());
    const editedDistributionFields = useRef(new Map<string, number>());
    const editedRightsFields = useRef(new Map<string, number>());
    const restrictedBroadcasters = useRef<{
      value: string[] | undefined;
    } | null>(null);
    const archiveDuration = useRef<{ value: number | undefined } | null>(null);
    const metadataLoad = useRef(0);
    const editRevision = useRef(0);

    // State for section toggles
    const [activeSection, setActiveSection] =
      useState<string>("contentWarnings");

    const currentYear = new Date().getFullYear();

    const getBroadcasterDID = useGetBroadcasterDID();
    useEffect(() => {
      getBroadcasterDID();
    }, [getBroadcasterDID]);

    const broadcasterDID = useStreamplaceStore((state) => state.broadcasterDID);

    // Load existing metadata on mount or from initialMetadata prop
    useEffect(() => {
      const request = ++metadataLoad.current;
      if (initialMetadata) {
        // Use provided initial metadata
        if (initialMetadata.contentWarnings?.warnings) {
          setContentWarnings(initialMetadata.contentWarnings.warnings);
        }
        if (initialMetadata.distributionPolicy) {
          setDistributionPolicy(initialMetadata.distributionPolicy);
        }
        setLoadingDistribution(false);
        if (initialMetadata.contentRights) {
          setContentRights(initialMetadata.contentRights);
          setSelectedLicense(initialMetadata.contentRights.license || "");
        }
        return;
      }

      const loadMetadata = async () => {
        if (!pdsAgent || !did) return;

        if (allSections) {
          setLoadingDistribution(true);
          setDistributionLoadError(false);
        }
        try {
          const metadata = await getContentMetadata();
          if (
            (allSections || preserveWarnings) &&
            request !== metadataLoad.current
          )
            return;
          if (metadata?.record) {
            setHasMetadata(true);
            if (preserveWarnings) {
              setContentWarnings((previous) =>
                mergeWarningChoices(
                  metadata.record.contentWarnings?.warnings,
                  previous,
                  editedWarnings.current,
                ),
              );
            } else if (
              metadata.record.contentWarnings?.warnings &&
              !editedSections.current.has("warnings")
            ) {
              setContentWarnings(
                metadata.record.contentWarnings.warnings as string[],
              );
            }
            const loadedDistribution = metadata.record.distributionPolicy;
            if (loadedDistribution) {
              if (allSections) {
                setDistributionPolicy((previous) =>
                  mergeEditedFields(
                    loadedDistribution,
                    previous,
                    editedDistributionFields.current,
                  ),
                );
              } else if (!editedSections.current.has("distribution")) {
                setDistributionPolicy(loadedDistribution);
              }
            }
            const loadedRights = metadata.record.contentRights;
            if (loadedRights) {
              if (allSections) {
                setContentRights((previous) =>
                  mergeEditedFields(
                    loadedRights,
                    previous,
                    editedRightsFields.current,
                  ),
                );
                if (!editedRightsFields.current.has("license")) {
                  setSelectedLicense(loadedRights.license || "");
                }
              } else if (!editedSections.current.has("rights")) {
                setContentRights(loadedRights);
                setSelectedLicense(loadedRights.license || "");
              }
            }
          }
        } catch (error) {
          if (allSections) {
            if (request === metadataLoad.current)
              setDistributionLoadError(true);
          } else {
            // No existing metadata is fine
            console.log("No existing metadata found");
          }
        } finally {
          if (allSections && request === metadataLoad.current) {
            setLoadingDistribution(false);
          }
        }
      };

      loadMetadata();
      return () => {
        if (
          (allSections || preserveWarnings) &&
          request === metadataLoad.current
        ) {
          metadataLoad.current++;
        }
      };
    }, [
      pdsAgent,
      did,
      initialMetadata,
      allSections,
      preserveWarnings,
      metadataRetry,
    ]);

    const handleContentWarningChange = useCallback(
      (warning: string, checked: boolean) => {
        editedSections.current.add("warnings");
        editRevision.current++;
        if (preserveWarnings)
          editedWarnings.current.set(warning, editRevision.current);
        const newWarnings = checked
          ? [...contentWarnings, warning]
          : contentWarnings.filter((w) => w !== warning);

        setContentWarnings(newWarnings);

        if (onMetadataChange) {
          onMetadataChange({
            $type: "place.stream.metadata.configuration",
            contentWarnings: { warnings: newWarnings },
            distributionPolicy,
            contentRights,
          });
        }
      },
      [
        contentWarnings,
        distributionPolicy,
        contentRights,
        onMetadataChange,
        preserveWarnings,
      ],
    );

    // Notify parent component when metadata changes
    useEffect(() => {
      if (onMetadataChange) {
        onMetadataChange({
          $type: "place.stream.metadata.configuration",
          contentWarnings: { warnings: contentWarnings },
          distributionPolicy,
          contentRights,
        });
      }
    }, [contentWarnings, distributionPolicy, contentRights, onMetadataChange]);

    // Handle distribution policy changes
    const handleDistributionPolicyChange = useCallback(
      (changes: Partial<place.stream.metadata.distributionPolicy.Main>) => {
        editedSections.current.add("distribution");
        editRevision.current++;
        for (const field of Object.keys(changes)) {
          editedDistributionFields.current.set(field, editRevision.current);
        }
        const newDistributionPolicy = { ...distributionPolicy, ...changes };
        setDistributionPolicy(newDistributionPolicy);

        if (onMetadataChange) {
          onMetadataChange({
            $type: "place.stream.metadata.configuration",
            contentWarnings: { warnings: contentWarnings },
            distributionPolicy: newDistributionPolicy,
            contentRights,
          });
        }
      },
      [
        contentWarnings,
        contentRights,
        onMetadataChange,
        distributionPolicy,
        setDistributionPolicy,
      ],
    );

    // Handle content rights changes
    const handleContentRightsChange = useCallback(
      (field: string, value: any) => {
        editedSections.current.add("rights");
        editRevision.current++;
        editedRightsFields.current.set(field, editRevision.current);
        const newRights = { ...contentRights, [field]: value };
        setContentRights(newRights);

        if (onMetadataChange) {
          onMetadataChange({
            $type: "place.stream.metadata.configuration",
            contentWarnings: { warnings: contentWarnings },
            distributionPolicy,
            contentRights: newRights,
          });
        }
      },
      [contentWarnings, distributionPolicy, contentRights, onMetadataChange],
    );

    const handleSave = useCallback(async () => {
      setLoading(true);
      try {
        const revision = editRevision.current;
        const edits = new Set(editedSections.current);
        const warningEdits = new Map(editedWarnings.current);
        const distributionEdits = new Map(editedDistributionFields.current);
        const rightsEdits = new Map(editedRightsFields.current);
        const current = await getContentMetadata();
        const warningsToSave =
          preserveWarnings && current?.record
            ? mergeWarningChoices(
                current.record.contentWarnings?.warnings,
                contentWarnings,
                warningEdits,
              )
            : contentWarnings;
        const reconcileWarnings = (saved: string[] | undefined) => {
          clearSavedFields(editedWarnings.current, warningEdits);
          const pending = new Map(editedWarnings.current);
          if (pending.size === 0) editedSections.current.delete("warnings");
          setContentWarnings((previous) =>
            mergeWarningChoices(saved, previous, pending),
          );
        };
        if (warningsOnly) {
          const savedWarnings =
            edits.has("warnings") || !current?.record
              ? warningsToSave
              : current.record.contentWarnings?.warnings;
          await saveContentMetadata(
            {
              ...current?.record,
              $type: "place.stream.metadata.configuration",
              contentWarnings:
                edits.has("warnings") || !current?.record
                  ? warningsToSave.length
                    ? {
                        ...current?.record?.contentWarnings,
                        warnings: warningsToSave,
                      }
                    : undefined
                  : current.record.contentWarnings,
            },
            current,
          );
          if (preserveWarnings) {
            metadataLoad.current++;
            reconcileWarnings(savedWarnings);
          }
          if (revision === editRevision.current) {
            editedSections.current.clear();
            if (!preserveWarnings && !edits.has("warnings"))
              setContentWarnings(
                current?.record?.contentWarnings?.warnings ?? [],
              );
          }
          setHasMetadata(true);
          toast.show(
            "Content metadata updated",
            "Your settings have been saved successfully",
          );
          return;
        }
        // Build the metadata object, only including non-empty fields
        const metadata: place.stream.metadata.configuration.Main = {
          ...current?.record,
          $type: "place.stream.metadata.configuration",
        };

        // Only include contentWarnings if it has values
        if (!current?.record || edits.has("warnings")) {
          metadata.contentWarnings = warningsToSave.length
            ? { ...current?.record?.contentWarnings, warnings: warningsToSave }
            : undefined;
        }

        // Only include contentRights if it has actual values
        const rightsWithLicense = {
          ...contentRights,
          license:
            selectedLicense === "custom"
              ? customLicenseText
              : selectedLicense || undefined,
        };

        // Filter out empty values from contentRights and convert copyrightYear to number
        const rightsToSave =
          current?.record && allSections
            ? mergeEditedFields(
                current.record.contentRights,
                rightsWithLicense,
                rightsEdits,
              )
            : rightsWithLicense;
        const filteredRights = Object.fromEntries(
          Object.entries(rightsToSave)
            .filter(
              ([_, value]) =>
                value !== undefined && value !== null && value !== "",
            )
            .map(([key, value]) => {
              // Convert copyrightYear to integer as per lexicon
              if (key === "copyrightYear" && typeof value === "string") {
                const year = parseInt(value, 10);
                return [key, isNaN(year) ? undefined : year];
              }
              return [key, value];
            })
            .filter(([_, value]) => value !== undefined),
        );

        if (!current?.record || edits.has("rights")) {
          metadata.contentRights =
            Object.keys(filteredRights).length > 0 ? filteredRights : undefined;
        }

        // AI training is opt-in: absent an explicit choice, save the record
        // with training disallowed.
        if (!current?.record || edits.has("distribution")) {
          const nextPolicy: place.stream.metadata.distributionPolicy.Main = {
            allowGenAiTraining: false,
            ...(!current?.record || !allSections
              ? distributionPolicy
              : mergeEditedFields(
                  current.record.distributionPolicy,
                  distributionPolicy,
                  distributionEdits,
                )),
          };

          if (nextPolicy.allowedBroadcasters) {
            nextPolicy.allowedBroadcasters =
              nextPolicy.allowedBroadcasters.filter(
                (broadcaster) => broadcaster !== "",
              );
            if (!allSections && revision === editRevision.current) {
              setDistributionPolicy({
                ...distributionPolicy,
                allowedBroadcasters: nextPolicy.allowedBroadcasters,
              });
            }
          }
          metadata.distributionPolicy = nextPolicy;
        }

        await saveContentMetadata(metadata, current);
        if (preserveWarnings)
          reconcileWarnings(metadata.contentWarnings?.warnings);
        if (allSections) {
          metadataLoad.current++;
          setLoadingDistribution(false);
          setDistributionLoadError(false);
          clearSavedFields(editedDistributionFields.current, distributionEdits);
          const pending = editedDistributionFields.current;
          if (pending.size === 0) editedSections.current.delete("distribution");
          setDistributionPolicy((previous) =>
            mergeEditedFields(metadata.distributionPolicy, previous, pending),
          );
          clearSavedFields(editedRightsFields.current, rightsEdits);
          const pendingRights = editedRightsFields.current;
          if (pendingRights.size === 0) editedSections.current.delete("rights");
          setContentRights((previous) =>
            mergeEditedFields(metadata.contentRights, previous, pendingRights),
          );
          if (!pendingRights.has("license") && !rightsEdits.has("license")) {
            setSelectedLicense(metadata.contentRights?.license || "");
          }
        }
        if (revision === editRevision.current) {
          editedSections.current.clear();
          editedWarnings.current.clear();
          editedDistributionFields.current.clear();
          editedRightsFields.current.clear();
          if (!preserveWarnings && !edits.has("warnings"))
            setContentWarnings(
              current?.record?.contentWarnings?.warnings ?? [],
            );
        }
        setHasMetadata(true);
        // Show success toast
        toast.show(
          hasMetadata ? "Content metadata updated" : "Content metadata created",
          "Your settings have been saved successfully",
        );
      } catch (error) {
        if (
          error instanceof Error &&
          "error" in error &&
          error.error === "InvalidSwap"
        ) {
          toast.show(
            "Settings changed elsewhere",
            "Save again to apply your edits.",
          );
        } else {
          console.error("Failed to save metadata:", error);
          toast.show("Failed to save metadata", "Please try again later");
        }
      } finally {
        setLoading(false);
      }
    }, [
      contentWarnings,
      contentRights,
      distributionPolicy,
      selectedLicense,
      customLicenseText,
      hasMetadata,
      warningsOnly,
      allSections,
      preserveWarnings,
      getContentMetadata,
      saveContentMetadata,
      toast,
    ]);

    return (
      <>
        <Container
          ref={ref}
          style={[scrollable && { flex: 1 }, style]}
          {...(scrollable && {
            showsVerticalScrollIndicator: false,
            contentContainerStyle: { flexGrow: 1 },
          })}
        >
          <View style={[gap.all[8], w.percent[100], { alignItems: "stretch" }]}>
            {/* Section Selector */}
            {!warningsOnly && !allSections && (
              <View style={[gap.all[4], w.percent[100]]}>
                <SegmentedTabs
                  options={[
                    { label: "Warnings", value: "contentWarnings" },
                    { label: "Rights", value: "contentRights" },
                    { label: "Distribution", value: "distribution" },
                  ]}
                  value={activeSection}
                  onChange={setActiveSection}
                />
              </View>
            )}

            {/* Content Warnings Section */}
            {(allSections || activeSection === "contentWarnings") && (
              <View style={[gap.all[3], w.percent[100]]}>
                <View
                  style={[
                    layout.flex.row,
                    layout.flex.alignCenter,
                    w.percent[100],
                    gap.all[2],
                  ]}
                >
                  <Text size="lg">Content Warnings</Text>
                </View>
                <View
                  style={[
                    gap.all[2],
                    w.percent[100],
                    allSections &&
                      width >= breakpoints.lg && {
                        flexDirection: "row",
                        flexWrap: "wrap",
                        columnGap: th.theme.spacing[0],
                      },
                  ]}
                >
                  {CONTENT_WARNINGS.map((warning) => {
                    const choice = (
                      <Checkbox
                        checked={contentWarnings.includes(warning.value)}
                        onCheckedChange={(checked) =>
                          handleContentWarningChange(warning.value, checked)
                        }
                        label={warning.label}
                        description={
                          showWarningDescriptions
                            ? warning.description
                            : undefined
                        }
                        style={[{ fontSize: 12 }]}
                      />
                    );
                    return (
                      <View
                        key={warning.value}
                        style={[
                          w.percent[100],
                          Platform.OS === "web" &&
                            !showWarningDescriptions &&
                            layout.flex.row,
                          Platform.OS === "web" &&
                            !showWarningDescriptions &&
                            layout.flex.alignCenter,
                          allSections &&
                            width >= breakpoints.lg && {
                              width: "50%",
                              paddingRight: th.theme.spacing[2],
                            },
                        ]}
                      >
                        {showWarningDescriptions ? (
                          choice
                        ) : (
                          <Tooltip
                            content={warning.description}
                            position="top"
                            style={
                              Platform.OS === "web"
                                ? { flexShrink: 1 }
                                : undefined
                            }
                          >
                            {choice}
                          </Tooltip>
                        )}
                        {Platform.OS === "web" && !showWarningDescriptions && (
                          <ExplanationDialog
                            compact
                            title={`About ${warning.label}`}
                            entries={[warning]}
                          />
                        )}
                      </View>
                    );
                  })}
                </View>
                <Admonition variant="info" size="sm">
                  <Text size="sm">
                    You are required to disclose if your content is not suitable
                    for certain viewers.
                  </Text>
                </Admonition>
                <Admonition variant="warning" size="sm">
                  <Text size="sm">
                    Your node may prohibit some of this content. Read their
                    community guidelines to make sure.{" "}
                    <Pressable
                      onPress={() =>
                        Linking.openURL(
                          "https://blog.stream.place/3mcqwibo4ks2w",
                        )
                      }
                    >
                      <Text size="sm" color={th.theme.colors.primary}>
                        Learn more{" "}
                        <ExternalLink
                          size="14"
                          style={{ marginVertical: -2 }}
                        />
                      </Text>
                    </Pressable>
                  </Text>
                </Admonition>
              </View>
            )}

            {/* Content Rights Section */}
            {(allSections || activeSection === "contentRights") && (
              <View style={[gap.all[3], w.percent[100]]}>
                <View
                  style={[
                    layout.flex.row,
                    layout.flex.alignCenter,
                    w.percent[100],
                    gap.all[2],
                  ]}
                >
                  <Text size={allSections ? "lg" : undefined}>
                    Content Rights
                  </Text>
                  <Text muted>(optional)</Text>
                </View>

                <View style={[gap.all[3], w.percent[100]]}>
                  <View
                    style={[
                      layout.flex.row,
                      layout.flex.alignCenter,
                      w.percent[100],
                      stackedFieldStyle,
                    ]}
                  >
                    <Text
                      nativeID={`${fieldIds}-year`}
                      style={[
                        text.neutral[300],
                        {
                          minWidth: 100,
                          textAlign: "left",
                          paddingBottom: 8,
                          fontSize: 14,
                        },
                        fieldLabelStyle,
                      ]}
                    >
                      Copyright Year
                    </Text>
                    <View
                      style={[flex.values[1], stackedFields && { flex: 0 }]}
                    >
                      <Input
                        aria-labelledby={`${fieldIds}-year`}
                        value={contentRights.copyrightYear?.toString() || ""}
                        onChange={(value) =>
                          handleContentRightsChange("copyrightYear", value)
                        }
                        placeholder={currentYear.toString()}
                        variant="filled"
                        inputStyle={[
                          p[3],
                          r.md,
                          bg.neutral[800],
                          text.white,
                          borders.width.thin,
                          borders.color.neutral[600],
                          w.percent[100],
                        ]}
                      />
                    </View>
                  </View>

                  <View
                    style={[
                      layout.flex.row,
                      layout.flex.alignCenter,
                      w.percent[100],
                      stackedFieldStyle,
                    ]}
                  >
                    <Text
                      nativeID={`${fieldIds}-license`}
                      style={[
                        text.neutral[300],
                        {
                          minWidth: 100,
                          textAlign: "left",
                          paddingBottom: 8,
                          fontSize: 14,
                        },
                        fieldLabelStyle,
                      ]}
                    >
                      License
                    </Text>
                    <View
                      style={[flex.values[1], stackedFields && { flex: 0 }]}
                    >
                      <Select
                        labelledBy={`${fieldIds}-license`}
                        value={selectedLicense}
                        onValueChange={(value) => {
                          setSelectedLicense(value);
                          handleContentRightsChange(
                            "license",
                            value === "custom" ? customLicenseText : value,
                          );
                        }}
                        placeholder="Select a license"
                        items={LICENSE_OPTIONS.map((opt) => ({
                          label: opt.label,
                          value: opt.value,
                          description: opt.description,
                        }))}
                        style={[
                          p[3],
                          r.md,
                          bg.neutral[800],
                          text.white,
                          borders.width.thin,
                          borders.color.neutral[600],
                          w.percent[100],
                        ]}
                      />
                    </View>
                  </View>

                  {/* Custom License Text Input */}
                  {selectedLicense === "custom" && (
                    <View
                      style={[
                        layout.flex.row,
                        layout.flex.alignCenter,
                        w.percent[100],
                        stackedFieldStyle,
                      ]}
                    >
                      <Text
                        nativeID={`${fieldIds}-custom-license`}
                        style={[
                          text.neutral[300],
                          {
                            minWidth: 100,
                            textAlign: "left",
                            paddingBottom: 8,
                            fontSize: 14,
                          },
                          fieldLabelStyle,
                        ]}
                      >
                        Custom License
                      </Text>
                      <View
                        style={[flex.values[1], stackedFields && { flex: 0 }]}
                      >
                        <Textarea
                          aria-labelledby={`${fieldIds}-custom-license`}
                          value={customLicenseText}
                          onChangeText={(value) => {
                            setCustomLicenseText(value);
                            if (selectedLicense === "custom") {
                              handleContentRightsChange("license", value);
                            }
                          }}
                          placeholder="Enter your custom license terms..."
                          style={[
                            p[3],
                            r.md,
                            bg.neutral[800],
                            text.white,
                            borders.width.thin,
                            borders.color.neutral[600],
                            w.percent[100],
                            stackedFields && {
                              flex: 0,
                              minHeight: th.theme.spacing[24],
                            },
                          ]}
                        />
                      </View>
                    </View>
                  )}

                  <View
                    style={[
                      layout.flex.row,
                      layout.flex.alignCenter,
                      w.percent[100],
                      stackedFieldStyle,
                    ]}
                  >
                    <Text
                      nativeID={`${fieldIds}-notice`}
                      style={[
                        text.neutral[300],
                        {
                          minWidth: 100,
                          textAlign: "left",
                          paddingBottom: 8,
                          fontSize: 14,
                        },
                        fieldLabelStyle,
                      ]}
                    >
                      Copyright Notice
                    </Text>
                    <View
                      style={[flex.values[1], stackedFields && { flex: 0 }]}
                    >
                      <Textarea
                        aria-labelledby={`${fieldIds}-notice`}
                        value={contentRights.copyrightNotice || ""}
                        onChangeText={(value: string) =>
                          handleContentRightsChange("copyrightNotice", value)
                        }
                        placeholder="Enter your copyright notice..."
                        style={[
                          p[3],
                          r.md,
                          bg.neutral[800],
                          text.white,
                          borders.width.thin,
                          borders.color.neutral[600],
                          w.percent[100],
                          stackedFields && {
                            flex: 0,
                            minHeight: th.theme.spacing[24],
                          },
                        ]}
                      />
                    </View>
                  </View>

                  <View
                    style={[
                      layout.flex.row,
                      layout.flex.alignCenter,
                      w.percent[100],
                      stackedFieldStyle,
                    ]}
                  >
                    <Text
                      nativeID={`${fieldIds}-credit`}
                      style={[
                        text.neutral[300],
                        {
                          minWidth: 100,
                          textAlign: "left",
                          paddingBottom: 8,
                          fontSize: 14,
                        },
                        fieldLabelStyle,
                      ]}
                    >
                      Credit Line
                    </Text>
                    <View
                      style={[flex.values[1], stackedFields && { flex: 0 }]}
                    >
                      <Textarea
                        aria-labelledby={`${fieldIds}-credit`}
                        value={contentRights.creditLine || ""}
                        onChangeText={(value: string) =>
                          handleContentRightsChange("creditLine", value)
                        }
                        placeholder="Enter your credit line..."
                        style={[
                          p[3],
                          r.md,
                          bg.neutral[800],
                          text.white,
                          borders.width.thin,
                          borders.color.neutral[600],
                          w.percent[100],
                          stackedFields && {
                            flex: 0,
                            minHeight: th.theme.spacing[24],
                          },
                        ]}
                      />
                    </View>
                  </View>
                </View>
              </View>
            )}

            {/* Distribution Section */}
            {(allSections || activeSection === "distribution") && (
              <View style={[gap.all[3], w.percent[100]]}>
                <View
                  style={[
                    layout.flex.row,
                    layout.flex.alignCenter,
                    w.percent[100],
                    gap.all[2],
                  ]}
                >
                  <Text size={allSections ? "lg" : undefined}>
                    Distribution
                  </Text>
                  <Text muted>(optional)</Text>
                  {Platform.OS === "web" && (
                    <ExplanationDialog
                      title="About distribution"
                      entries={distributionExplanations}
                    />
                  )}
                </View>
                {allSections && loadingDistribution && (
                  <Text size="xs" color="muted">
                    Loading distribution settings…
                  </Text>
                )}
                {allSections && distributionLoadError && (
                  <View style={[gap.all[2], { alignItems: "flex-start" }]}>
                    <Text
                      size="sm"
                      color="destructive"
                      accessibilityRole="alert"
                    >
                      Could not load distribution settings.
                    </Text>
                    <Button
                      size="sm"
                      width="min"
                      variant="secondary"
                      accessibilityLabel="Retry distribution settings"
                      onPress={() => setMetadataRetry((value) => value + 1)}
                    >
                      Retry
                    </Button>
                  </View>
                )}

                {/* allow everyone to distribute your content */}
                <Tooltip
                  content={distributionExplanations[0].description}
                  position="top"
                >
                  <Checkbox
                    disabled={
                      allSections &&
                      (loadingDistribution || distributionLoadError)
                    }
                    checked={
                      distributionPolicy.allowedBroadcasters?.includes("*") ||
                      false
                    }
                    onCheckedChange={(checked) => {
                      if (allSections && checked) {
                        restrictedBroadcasters.current = {
                          value: distributionPolicy.allowedBroadcasters,
                        };
                      }
                      const existing =
                        distributionPolicy.allowedBroadcasters?.filter(
                          (broadcaster) => broadcaster !== "*",
                        );
                      handleDistributionPolicyChange({
                        allowedBroadcasters: checked
                          ? ["*"]
                          : allSections && restrictedBroadcasters.current
                            ? restrictedBroadcasters.current.value
                            : allSections && existing?.length
                              ? existing
                              : [broadcasterDID || ""],
                      });
                    }}
                    label={"Allow everyone to distribute your content"}
                    style={[{ fontSize: 12 }]}
                  />
                </Tooltip>

                {/* allowedBroadcasters */}
                {!distributionPolicy.allowedBroadcasters?.includes("*") && (
                  <View style={[gap.all[3], w.percent[100]]}>
                    <View
                      style={[
                        layout.flex.row,
                        layout.flex.alignCenter,
                        w.percent[100],
                        stackedFieldStyle,
                      ]}
                    >
                      <Text
                        nativeID={`${fieldIds}-broadcasters`}
                        leading="tight"
                        style={[
                          text.neutral[300],
                          {
                            minWidth: 100,
                            maxWidth: 100,
                            textAlign: "left",
                            paddingBottom: 8,
                            fontSize: 14,
                          },
                          fieldLabelStyle,
                        ]}
                      >
                        Allowed Broadcasters
                      </Text>
                      <View
                        style={[flex.values[1], stackedFields && { flex: 0 }]}
                      >
                        <Text
                          style={[
                            text.gray[500],
                            { fontSize: 12, paddingBottom: 4 },
                          ]}
                        >
                          Enter the did:webs of the broadcasters you want to
                          allow to distribute your content, one per line.
                        </Text>
                        <Input
                          aria-labelledby={`${fieldIds}-broadcasters`}
                          disabled={allSections && distributionLoadError}
                          multiline={true}
                          numberOfLines={4}
                          value={
                            distributionPolicy.allowedBroadcasters?.join(
                              "\n",
                            ) || ""
                          }
                          onChange={(value) => {
                            handleDistributionPolicyChange({
                              allowedBroadcasters: value.split("\n"),
                            });
                          }}
                          variant="filled"
                          inputStyle={[
                            p[3],
                            r.md,
                            bg.neutral[800],
                            text.white,
                            borders.width.thin,
                            borders.color.neutral[600],
                            w.percent[100],
                          ]}
                        />
                      </View>
                    </View>
                  </View>
                )}

                <Tooltip
                  content={distributionExplanations[1].description}
                  position="top"
                >
                  <Checkbox
                    disabled={
                      allSections &&
                      (loadingDistribution || distributionLoadError)
                    }
                    checked={distributionPolicy.allowGenAiTraining === true}
                    onCheckedChange={(checked) =>
                      handleDistributionPolicyChange({
                        allowGenAiTraining: checked,
                      })
                    }
                    label={
                      "Mark your content as available for generative AI training"
                    }
                    style={[{ fontSize: 12 }]}
                  />
                </Tooltip>

                <Tooltip
                  content={distributionExplanations[2].description}
                  position="top"
                >
                  <Checkbox
                    disabled={
                      allSections &&
                      (loadingDistribution || distributionLoadError)
                    }
                    checked={distributionPolicy.deleteAfter === -1}
                    onCheckedChange={(checked) => {
                      if (allSections && checked) {
                        archiveDuration.current = {
                          value: distributionPolicy.deleteAfter,
                        };
                      }
                      handleDistributionPolicyChange({
                        deleteAfter: checked
                          ? -1
                          : allSections && archiveDuration.current
                            ? archiveDuration.current.value
                            : 300,
                      });
                    }}
                    label={"Allow everyone to archive your content"}
                    style={[{ fontSize: 12 }]}
                  />
                </Tooltip>

                {/* deleteAfter */}
                {distributionPolicy.deleteAfter !== -1 && (
                  <View style={[gap.all[3], w.percent[100]]}>
                    <View
                      style={[
                        layout.flex.row,
                        layout.flex.alignCenter,
                        w.percent[100],
                        stackedFieldStyle,
                      ]}
                    >
                      <Text
                        nativeID={`${fieldIds}-retention`}
                        style={[
                          text.neutral[300],
                          {
                            minWidth: 100,
                            textAlign: "left",
                            paddingBottom: 8,
                            fontSize: 14,
                          },
                          fieldLabelStyle,
                        ]}
                      >
                        Delete After
                      </Text>
                      <View
                        style={[flex.values[1], stackedFields && { flex: 0 }]}
                      >
                        <Text
                          style={[
                            text.gray[500],
                            { fontSize: 12, paddingBottom: 4 },
                          ]}
                        >
                          Duration in seconds (e.g., 300 for 5 minutes)
                        </Text>
                        <Input
                          aria-labelledby={`${fieldIds}-retention`}
                          disabled={allSections && distributionLoadError}
                          value={
                            distributionPolicy.deleteAfter?.toString() || ""
                          }
                          onChange={(value) => {
                            const duration = parseInt(value, 10);
                            handleDistributionPolicyChange({
                              deleteAfter:
                                isNaN(duration) || duration === 0
                                  ? undefined
                                  : Math.max(-1, duration),
                            });
                          }}
                          keyboardType="numeric"
                          variant="filled"
                          inputStyle={[
                            p[3],
                            r.md,
                            bg.neutral[800],
                            text.white,
                            borders.width.thin,
                            borders.color.neutral[600],
                            w.percent[100],
                          ]}
                        />
                      </View>
                    </View>
                  </View>
                )}
              </View>
            )}

            {/* Save Button - Always visible */}
            <View style={[layout.flex.center, w.percent[100]]}>
              <Button
                onPress={handleSave}
                loading={loading}
                disabled={loading}
                style={[
                  bg.primary[500],
                  r.md,
                  gap.all[3],
                  { minWidth: 200 },
                  layout.flex.center,
                  { opacity: loading ? 0.5 : 1 },
                ]}
              >
                <Text
                  style={[
                    text.white,
                    w.percent[100],
                    { fontSize: 16, fontWeight: "bold" },
                  ]}
                >
                  {hasMetadata ? "Update Metadata" : "Save Metadata"}
                </Text>
              </Button>
            </View>
          </View>
        </Container>
      </>
    );
  },
);

ContentMetadataForm.displayName = "ContentMetadataForm";
