import { forwardRef, useCallback, useEffect, useState } from "react";
import { ScrollView, View } from "react-native";
import {
  CONTENT_WARNINGS,
  LICENSE_OPTIONS,
} from "../../lib/metadata-constants";

import {
  PlaceStreamMetadataConfiguration,
  PlaceStreamMetadataContentRights,
  PlaceStreamMetadataDistributionPolicy,
} from "streamplace";
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
import { Button } from "../ui/button";
import { Checkbox } from "../ui/checkbox";
import { Input } from "../ui/input";
import { Select } from "../ui/select";
import { Text } from "../ui/text";
import { Textarea } from "../ui/textarea";
import { useToast } from "../ui/toast";
import { Tooltip } from "../ui/tooltip";

const { p, r, bg, borders, w, text, layout, gap, flex } = zero;

export interface ContentMetadataFormProps {
  showUpdateButton?: boolean;
  onMetadataChange?: (
    metadata: PlaceStreamMetadataConfiguration.Record,
  ) => void;
  initialMetadata?: PlaceStreamMetadataConfiguration.Record;
  style?: any;
}

// ButtonSelector component (same as in livestream-panel)
const ButtonSelector = ({
  values,
  selectedValue,
  setSelectedValue,
  disabledValues = [],
  style = [],
}: {
  values: { label: string; value: string }[];
  selectedValue: string;
  setSelectedValue: (value: string) => void;
  disabledValues?: string[];
  style?: any[];
}) => (
  <View style={[layout.flex.row, gap.all[1], ...style]}>
    {values.map(({ label, value }) => (
      <Button
        key={value}
        variant={selectedValue === value ? "primary" : "secondary"}
        size="pill"
        disabled={disabledValues.includes(value)}
        onPress={() => setSelectedValue(value)}
        style={[
          r.md,
          {
            opacity: disabledValues.includes(value) ? 0.5 : 1,
          },
        ]}
      >
        <Text
          style={[
            selectedValue === value ? text.white : text.gray[300],
            { fontSize: 14, fontWeight: "600" },
          ]}
        >
          {label}
        </Text>
      </Button>
    ))}
  </View>
);

export const ContentMetadataForm = forwardRef<any, ContentMetadataFormProps>(
  (
    { showUpdateButton = false, onMetadataChange, initialMetadata, style },
    ref,
  ) => {
    const pdsAgent = usePDSAgent();
    const did = useDID();
    const getContentMetadata = useGetContentMetadata();
    const saveContentMetadata = useSaveContentMetadata();
    const toast = useToast();

    // Local state for metadata
    const [contentWarnings, setContentWarnings] = useState<string[]>([]);
    const [distributionPolicy, setDistributionPolicy] =
      useState<PlaceStreamMetadataDistributionPolicy.Main>({});
    const [contentRights, setContentRights] =
      useState<PlaceStreamMetadataContentRights.Main>({});
    const [selectedLicense, setSelectedLicense] = useState<string>("");
    const [customLicenseText, setCustomLicenseText] = useState<string>("");
    const [loading, setLoading] = useState(false);
    const [hasMetadata, setHasMetadata] = useState(false);

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
      if (initialMetadata) {
        // Use provided initial metadata
        if (initialMetadata.contentWarnings?.warnings) {
          setContentWarnings(initialMetadata.contentWarnings.warnings);
        }
        if (initialMetadata.distributionPolicy) {
          setDistributionPolicy(initialMetadata.distributionPolicy);
        }
        if (initialMetadata.contentRights) {
          setContentRights(initialMetadata.contentRights);
          setSelectedLicense(initialMetadata.contentRights.license || "");
        }
        return;
      }

      const loadMetadata = async () => {
        if (!pdsAgent || !did) return;

        try {
          const metadata = await getContentMetadata();
          if (metadata?.record) {
            setHasMetadata(true);
            if (metadata.record.contentWarnings?.warnings) {
              setContentWarnings(
                metadata.record.contentWarnings.warnings as string[],
              );
            }
            if (metadata.record.distributionPolicy) {
              setDistributionPolicy(metadata.record.distributionPolicy);
            }
            if (metadata.record.contentRights) {
              setContentRights(metadata.record.contentRights);
              setSelectedLicense(metadata.record.contentRights.license || "");
            }
          }
        } catch (error) {
          // No existing metadata is fine
          console.log("No existing metadata found");
        }
      };

      loadMetadata();
    }, [pdsAgent, did, initialMetadata]);

    const handleContentWarningChange = useCallback(
      (warning: string, checked: boolean) => {
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
      [contentWarnings, distributionPolicy, contentRights, onMetadataChange],
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
      ({
        deleteAfter,
        allowedBroadcasters,
      }: {
        deleteAfter?: string;
        allowedBroadcasters?: string;
      }) => {
        let newDistributionPolicy: PlaceStreamMetadataDistributionPolicy.Main =
          {
            ...distributionPolicy,
          };
        if (typeof deleteAfter === "string") {
          let duration = parseInt(deleteAfter, 10);
          if (isNaN(duration)) {
            newDistributionPolicy.deleteAfter = undefined;
          } else {
            if (isNaN(duration) || duration < 0) {
              duration = -1;
            }
            newDistributionPolicy.deleteAfter =
              duration === 0 ? undefined : duration;
          }
        }
        if (typeof allowedBroadcasters === "string") {
          newDistributionPolicy.allowedBroadcasters =
            allowedBroadcasters.split("\n");
        }
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
        // Build the metadata object, only including non-empty fields
        const metadata: PlaceStreamMetadataConfiguration.Record = {
          $type: "place.stream.metadata.configuration",
        };

        // Only include contentWarnings if it has values
        if (contentWarnings && contentWarnings.length > 0) {
          metadata.contentWarnings = { warnings: contentWarnings };
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
        const filteredRights = Object.fromEntries(
          Object.entries(rightsWithLicense)
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

        if (Object.keys(filteredRights).length > 0) {
          metadata.contentRights = filteredRights;
        }

        metadata.distributionPolicy = {
          ...distributionPolicy,
        };

        if (distributionPolicy?.allowedBroadcasters) {
          const filteredBs = distributionPolicy.allowedBroadcasters.filter(
            (broadcaster) => broadcaster !== "",
          );
          metadata.distributionPolicy.allowedBroadcasters = filteredBs;
          setDistributionPolicy({
            ...distributionPolicy,
            allowedBroadcasters: filteredBs,
          });
        }

        await saveContentMetadata(metadata);
        setHasMetadata(true);
        // Show success toast
        toast.show(
          hasMetadata ? "Content metadata updated" : "Content metadata created",
          "Your settings have been saved successfully",
        );
      } catch (error) {
        console.error("Failed to save metadata:", error);
        // Show error toast
        toast.show("Failed to save metadata", "Please try again later");
      } finally {
        setLoading(false);
      }
    }, [
      contentWarnings,
      contentRights,
      selectedLicense,
      customLicenseText,
      hasMetadata,
      saveContentMetadata,
    ]);

    return (
      <>
        <ScrollView
          ref={ref}
          style={[{ flex: 1 }, style]}
          showsVerticalScrollIndicator={false}
          contentContainerStyle={{ flexGrow: 1 }}
        >
          <View style={[gap.all[8], w.percent[100], { alignItems: "stretch" }]}>
            {/* Section Selector */}
            <View style={[gap.all[4], w.percent[100]]}>
              <ButtonSelector
                values={[
                  { label: "Content Warnings", value: "contentWarnings" },
                  { label: "Content Rights", value: "contentRights" },
                  { label: "Distribution", value: "distribution" },
                ]}
                selectedValue={activeSection}
                setSelectedValue={setActiveSection}
                style={[{ marginVertical: -2, flexDirection: "column" }]}
              />
            </View>

            {/* Content Warnings Section */}
            {activeSection === "contentWarnings" && (
              <View style={[gap.all[3], w.percent[100]]}>
                <View
                  style={[
                    layout.flex.row,
                    layout.flex.alignCenter,
                    w.percent[100],
                    gap.all[2],
                  ]}
                >
                  <Text>Content Warnings</Text>
                  <Text muted>(optional)</Text>
                </View>
                <View style={[gap.all[2], w.percent[100]]}>
                  {CONTENT_WARNINGS.map((warning) => (
                    <View key={warning.value} style={[w.percent[100]]}>
                      <Tooltip content={warning.description} position="top">
                        <Checkbox
                          checked={contentWarnings.includes(warning.value)}
                          onCheckedChange={(checked) =>
                            handleContentWarningChange(warning.value, checked)
                          }
                          label={warning.label}
                          style={[{ fontSize: 12 }]}
                        />
                      </Tooltip>
                    </View>
                  ))}
                </View>
              </View>
            )}

            {/* Content Rights Section */}
            {activeSection === "contentRights" && (
              <View style={[gap.all[3], w.percent[100]]}>
                <View
                  style={[
                    layout.flex.row,
                    layout.flex.alignCenter,
                    w.percent[100],
                    gap.all[2],
                  ]}
                >
                  <Text>Content Rights</Text>
                  <Text muted>(optional)</Text>
                </View>

                <View style={[gap.all[3], w.percent[100]]}>
                  <View
                    style={[
                      layout.flex.row,
                      layout.flex.alignCenter,
                      w.percent[100],
                    ]}
                  >
                    <Text
                      style={[
                        text.neutral[300],
                        {
                          minWidth: 100,
                          textAlign: "left",
                          paddingBottom: 8,
                          fontSize: 14,
                        },
                      ]}
                    >
                      Copyright Year
                    </Text>
                    <View style={[flex.values[1]]}>
                      <Input
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
                    ]}
                  >
                    <Text
                      style={[
                        text.neutral[300],
                        {
                          minWidth: 100,
                          textAlign: "left",
                          paddingBottom: 8,
                          fontSize: 14,
                        },
                      ]}
                    >
                      License
                    </Text>
                    <View style={[flex.values[1]]}>
                      <Select
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
                      ]}
                    >
                      <Text
                        style={[
                          text.neutral[300],
                          {
                            minWidth: 100,
                            textAlign: "left",
                            paddingBottom: 8,
                            fontSize: 14,
                          },
                        ]}
                      >
                        Custom License
                      </Text>
                      <View style={[flex.values[1]]}>
                        <Textarea
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
                    ]}
                  >
                    <Text
                      style={[
                        text.neutral[300],
                        {
                          minWidth: 100,
                          textAlign: "left",
                          paddingBottom: 8,
                          fontSize: 14,
                        },
                      ]}
                    >
                      Copyright Notice
                    </Text>
                    <View style={[flex.values[1]]}>
                      <Textarea
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
                        ]}
                      />
                    </View>
                  </View>

                  <View
                    style={[
                      layout.flex.row,
                      layout.flex.alignCenter,
                      w.percent[100],
                    ]}
                  >
                    <Text
                      style={[
                        text.neutral[300],
                        {
                          minWidth: 100,
                          textAlign: "left",
                          paddingBottom: 8,
                          fontSize: 14,
                        },
                      ]}
                    >
                      Credit Line
                    </Text>
                    <View style={[flex.values[1]]}>
                      <Textarea
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
                        ]}
                      />
                    </View>
                  </View>
                </View>
              </View>
            )}

            {/* Distribution Section */}
            {activeSection === "distribution" && (
              <View style={[gap.all[3], w.percent[100]]}>
                <View
                  style={[
                    layout.flex.row,
                    layout.flex.alignCenter,
                    w.percent[100],
                    gap.all[2],
                  ]}
                >
                  <Text>Distribution</Text>
                  <Text muted>(optional)</Text>
                </View>

                {/* allow everyone to distribute your content */}
                <Tooltip
                  content="Distribution of your content is unlimited, but they still have to respect the deleteAfter policy below."
                  position="top"
                >
                  <Checkbox
                    checked={
                      distributionPolicy.allowedBroadcasters?.includes("*") ||
                      false
                    }
                    onCheckedChange={(checked) =>
                      handleDistributionPolicyChange({
                        allowedBroadcasters: checked
                          ? "*"
                          : broadcasterDID || "",
                      })
                    }
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
                      ]}
                    >
                      <Text
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
                        ]}
                      >
                        Allowed Broadcasters
                      </Text>
                      <View style={[flex.values[1]]}>
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
                          multiline={true}
                          numberOfLines={4}
                          value={
                            distributionPolicy.allowedBroadcasters?.join(
                              "\n",
                            ) || ""
                          }
                          onChange={(value) => {
                            handleDistributionPolicyChange({
                              allowedBroadcasters: value,
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
                  content="Anyone may archive your content indefinitely."
                  position="top"
                >
                  <Checkbox
                    checked={distributionPolicy.deleteAfter === -1}
                    onCheckedChange={(checked) => {
                      if (checked) {
                        handleDistributionPolicyChange({
                          deleteAfter: "-1",
                        });
                      } else {
                        handleDistributionPolicyChange({
                          deleteAfter: "300",
                        });
                      }
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
                      ]}
                    >
                      <Text
                        style={[
                          text.neutral[300],
                          {
                            minWidth: 100,
                            textAlign: "left",
                            paddingBottom: 8,
                            fontSize: 14,
                          },
                        ]}
                      >
                        Delete After
                      </Text>
                      <View style={[flex.values[1]]}>
                        <Text
                          style={[
                            text.gray[500],
                            { fontSize: 12, paddingBottom: 4 },
                          ]}
                        >
                          Duration in seconds (e.g., 300 for 5 minutes)
                        </Text>
                        <Input
                          value={
                            distributionPolicy.deleteAfter?.toString() || ""
                          }
                          onChange={(value) => {
                            handleDistributionPolicyChange({
                              deleteAfter: value,
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
        </ScrollView>
      </>
    );
  },
);

ContentMetadataForm.displayName = "ContentMetadataForm";
