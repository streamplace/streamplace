package media

import (
	"context"
	"encoding/json"
	"fmt"

	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/streamplace"
)

// ManifestBuilder is responsible for creating C2PA (Content Credentials) manifests
// for livestream segments.
// The builder creates manifests that include:
// - Basic livestream information (title, creator, date)
// - Content rights and copyright information
// - Content warnings for sensitive material
// - Distribution policies
// - C2PA action history (created, published)
// The manifest is meant to align closely with the IPTC Video Metadata Recommendations.
// See https://iptc.org/std/videometadatahub/recommendation/IPTC-VideoMetadataHub-props-Rec_1.6.html
type ManifestBuilder struct {
	model model.Model
}

func NewManifestBuilder(model model.Model) *ManifestBuilder {
	return &ManifestBuilder{
		model: model,
	}
}

func toObj(record any) (obj, error) {
	jsonBs, err := json.Marshal(record)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal record: %w", err)
	}
	var o obj
	err = json.Unmarshal(jsonBs, &o)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal record: %w", err)
	}
	return o, nil
}

func (mb *ManifestBuilder) BuildManifest(ctx context.Context, streamerName string, start int64) ([]byte, error) {
	log.Debug(ctx, "🔍 BuildManifest ENTRY", "streamer", streamerName, "start", start)
	// Start with base manifest
	startTime := aqtime.FromMillis(start).String()
	mani := obj{
		"title": fmt.Sprintf("Livestream Segment at %s", startTime),
		"assertions": []obj{
			// Required by spec, just basic info
			{
				"label": "c2pa.actions",
				"data": obj{
					"actions": []obj{
						{
							"action": "c2pa.created",
							"when":   startTime,
						},
					},
				},
			},
			// Content metadata, with extra custom fields added later
			{
				"label": "cawg.metadata",
				"data": obj{
					"@context": obj{
						"dc":          "http://purl.org/dc/elements/1.1/",
						"Iptc4xmpExt": "http://iptc.org/std/Iptc4xmpExt/2008-02-29/",
						"photoshop":   "http://ns.adobe.com/photoshop/1.0/",
						"xmpRights":   "http://ns.adobe.com/xap/1.0/rights/",
					},
					"dc:creator": streamerName,
					"dc:title":   "livestream",
					"dc:date":    startTime,
				},
			},
		},
	}

	// Add database metadata if available
	if mb.model != nil {
		metadata, err := mb.model.GetMetadataConfiguration(ctx, streamerName)
		if err != nil {
			log.Warn(ctx, "ManifestBuilder: failed to retrieve metadata", "error", err, "did", streamerName)
			return nil, fmt.Errorf("failed to retrieve metadata: %w", err)
		} else if metadata != nil {
			log.Debug(ctx, "ManifestBuilder: found metadata configuration", "did", streamerName, "metadata", metadata)
			streamplaceMetadata, err := metadata.ToStreamplaceMetadataConfiguration()
			if err != nil {
				log.Warn(ctx, "ManifestBuilder: failed to convert metadata, using defaults", "error", err, "did", streamerName)
			} else {
				log.Debug(ctx, "ManifestBuilder: enhancing manifest with metadata", "did", streamerName, "contentWarnings", streamplaceMetadata.ContentWarnings, "contentRights", streamplaceMetadata.ContentRights)
				mani = mb.enhanceManifestWithMetadata(mani, streamplaceMetadata, start)
				metadataObj, err := toObj(streamplaceMetadata)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal metadata: %w", err)
				}
				mani["assertions"] = append(mani["assertions"].([]obj), obj{
					"label": "place.stream.metadata.configuration",
					"data":  metadataObj,
				})
			}
		} else {
			log.Warn(ctx, "ManifestBuilder: no metadata configuration found for streamer", "did", streamerName)
		}
	}

	// Add livestream title if available
	livestreamTitle := "livestream" // default fallback
	if mb.model != nil {
		livestream, err := mb.model.GetLatestLivestreamForRepo(streamerName)
		if err != nil {
			log.Warn(ctx, "ManifestBuilder: failed to retrieve livestream, using default title", "error", err, "did", streamerName)
		} else if livestream != nil {
			// Extract title from livestream record
			livestreamRecord, err := livestream.ToLivestreamView()
			if err != nil {
				log.Warn(ctx, "ManifestBuilder: failed to convert livestream to view, using default title", "error", err, "did", streamerName)
			} else {
				if ls, ok := livestreamRecord.Record.Val.(*streamplace.Livestream); ok {
					livestreamTitle = ls.Title
					livestreamObj, err := toObj(ls)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal livestream: %w", err)
					}
					mani["assertions"] = append(mani["assertions"].([]obj), obj{
						"label": "place.stream.livestream",
						"data":  livestreamObj,
					})
				}
			}
		}
	}

	// Update the manifest title with the retrieved livestream title
	mani["assertions"].([]obj)[1]["data"].(obj)["dc:title"] = livestreamTitle

	// Convert manifest to JSON bytes for use with Rust c2pa library
	manifestBs, err := json.Marshal(mani)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	return manifestBs, nil
}

// getLicenseCodeMap returns a map of internal license codes to their corresponding URLs
func getLicenseCodeMap() map[string]string {
	return map[string]string{
		constants.LicenseCC0_1_0:      constants.LicenseURLCC0_1_0,
		constants.LicenseCCBy_4_0:     constants.LicenseURLCCBy_4_0,
		constants.LicenseCCBySA_4_0:   constants.LicenseURLCCBySA_4_0,
		constants.LicenseCCByNC_4_0:   constants.LicenseURLCCByNC_4_0,
		constants.LicenseCCByNCSA_4_0: constants.LicenseURLCCByNCSA_4_0,
		constants.LicenseCCByND_4_0:   constants.LicenseURLCCByND_4_0,
		constants.LicenseCCByNCND_4_0: constants.LicenseURLCCByNCND_4_0,
	}
}

// getWarningCodeMap returns a map of internal warning codes to their corresponding C2PA codes
func getWarningCodeMap() map[string]string {
	return map[string]string{
		constants.WarningDeath:           constants.WarningC2PADeath,
		constants.WarningDrugUse:         constants.WarningC2PADrugUse,
		constants.WarningFantasyViolence: constants.WarningC2PAFantasyViolence,
		constants.WarningFlashingLights:  constants.WarningC2PAFlashingLights,
		constants.WarningLanguage:        constants.WarningC2PALanguage,
		constants.WarningNudity:          constants.WarningC2PANudity,
		constants.WarningPII:             constants.WarningC2PAPII,
		constants.WarningSexuality:       constants.WarningC2PASexuality,
		constants.WarningSuffering:       constants.WarningC2PASuffering,
		constants.WarningViolence:        constants.WarningC2PAViolence,
	}
}

func (mb *ManifestBuilder) enhanceManifestWithMetadata(mani obj, metadata *streamplace.MetadataConfiguration, startTimeMillis int64) obj {
	if metadata.ContentRights != nil {
		// TODO: We are currently validating the creator in the ValidateMP4 function to be the streamer DID
		// if metadata.ContentRights.Creator != nil {
		//	mani["assertions"].([]obj)[1]["data"].(obj)["dc:creator"] = *metadata.ContentRights.Creator
		// }

		// Copyright Notice
		if metadata.ContentRights.CopyrightNotice != nil {
			mani["assertions"].([]obj)[1]["data"].(obj)["dc:rights"] = *metadata.ContentRights.CopyrightNotice
		}

		// Copyright Year
		if metadata.ContentRights.CopyrightYear != nil {
			mani["assertions"].([]obj)[1]["data"].(obj)["Iptc4xmpExt:CopyrightYear"] = *metadata.ContentRights.CopyrightYear
		}

		// Credit Line
		if metadata.ContentRights.CreditLine != nil {
			mani["assertions"].([]obj)[1]["data"].(obj)["photoshop:Credit"] = *metadata.ContentRights.CreditLine
		}

		// Build the license field
		if metadata.ContentRights.License != nil {
			// Map internal license codes to known licenses
			licenseCodeMap := getLicenseCodeMap()
			if mappedCode, exists := licenseCodeMap[*metadata.ContentRights.License]; exists {
				// it's a known linked license, so we can use the mapped code
				mani["assertions"].([]obj)[1]["data"].(obj)["Iptc4xmpExt:LinkedEncRightsExpr"] = mappedCode
			} else {
				// This is either an unknown or an unlinked license, so we need to put it in the UsageTerms field
				// which allows for licensing terms expressed in free text
				if *metadata.ContentRights.License == constants.LicenseAllRightsReserved {
					// if all rights reserved, we can put the string "All rights reserved" in the UsageTerms field
					mani["assertions"].([]obj)[1]["data"].(obj)["xmpRights:UsageTerms"] = "All rights reserved"
				} else {
					// it's an unknown license, so we need to put it directly in the UsageTerms field
					mani["assertions"].([]obj)[1]["data"].(obj)["xmpRights:UsageTerms"] = *metadata.ContentRights.License
				}
			}
		}
	}

	if metadata.ContentWarnings != nil && len(metadata.ContentWarnings.Warnings) > 0 {
		// Map internal warning codes to C2PA warning codes
		warningCodeMap := getWarningCodeMap()

		for i, warning := range metadata.ContentWarnings.Warnings {
			if mappedCode, exists := warningCodeMap[warning]; exists {
				metadata.ContentWarnings.Warnings[i] = mappedCode
			}
			// Unknown warnings remain unchanged
		}
		mani["assertions"].([]obj)[1]["data"].(obj)["Iptc4xmpExt:ContentWarning"] = metadata.ContentWarnings.Warnings
	}

	return mani
}
