package atproto

import (
	"strings"

	"stream.place/streamplace/pkg/comatproto"
)

const (
	// from com.atproto.label.defs
	LabelHide              = "!hide"
	LabelNoPromote         = "!no-promote"
	LabelWarn              = "!warn"
	LabelNoUnauthenticated = "!no-unauthenticated"
	LabelDMCAViolation     = "dmca-violation"
	LabelDoxxing           = "doxxing"
	LabelPorn              = "porn"
	LabelSexual            = "sexual"
	LabelNudity            = "nudity"
	LabelNSFL              = "nsfl"
	LabelGore              = "gore"

	// referenced in https://atproto.com/specs/label
	LabelTakedown = "!takedown"
	LabelSuspend  = "!suspend"

	// borrowed from moderation.bsky.app
	LabelSexualFigurative  = "sexual-figurative"
	LabelGraphicMedia      = "graphic-media"
	LabelSelfHarm          = "self-harm"
	LabelSensitive         = "sensitive"
	LabelExtremist         = "extremist"
	LabelIntolerant        = "intolerant"
	LabelThreat            = "threat"
	LabelRude              = "rude"
	LabelIllicit           = "illicit"
	LabelSecurity          = "security"
	LabelUnsafeLink        = "unsafe-link"
	LabelImpersonation     = "impersonation"
	LabelMisinformation    = "misinformation"
	LabelScam              = "scam"
	LabelEngagementFarming = "engagement-farming"
	LabelSpam              = "spam"
	LabelRumor             = "rumor"
	LabelMisleading        = "misleading"
	LabelInauthentic       = "inauthentic"

	// Streamplace-specific labels
	LabelNoViewers = "!no-viewers"
)

var bannedLabels = map[string]bool{
	LabelDMCAViolation:     true,
	LabelDoxxing:           true,
	LabelPorn:              true,
	LabelSexual:            true,
	LabelNudity:            true,
	LabelNSFL:              true,
	LabelGore:              true,
	LabelTakedown:          true,
	LabelSuspend:           true,
	LabelSexualFigurative:  true,
	LabelGraphicMedia:      true,
	LabelSelfHarm:          true,
	LabelSensitive:         true,
	LabelExtremist:         true,
	LabelIntolerant:        true,
	LabelThreat:            true,
	LabelRude:              true,
	LabelIllicit:           true,
	LabelSecurity:          true,
	LabelUnsafeLink:        true,
	LabelImpersonation:     true,
	LabelMisinformation:    true,
	LabelScam:              true,
	LabelEngagementFarming: true,
	LabelSpam:              true,
	LabelRumor:             true,
	LabelMisleading:        true,
	LabelInauthentic:       true,
}

// Given a users' labels, determine if they are banned
func IsBanned(labels ...*comatproto.LabelDefs_Label) bool {
	for _, l := range labels {
		if !strings.HasPrefix(l.Uri, "did:") {
			// this is a label on a record, not a user
			continue
		}
		if bannedLabels[l.Val] {
			return true
		}
	}
	return false
}

// IsViewerBanned returns true if the DID should be excluded from view count
// aggregation. This includes all standard banned labels plus !no-viewers.
func IsViewerBanned(labels ...*comatproto.LabelDefs_Label) bool {
	for _, l := range labels {
		if !strings.HasPrefix(l.Uri, "did:") {
			continue
		}
		if bannedLabels[l.Val] || l.Val == LabelNoViewers {
			return true
		}
	}
	return false
}

var chatHiddenLabels = map[string]bool{
	// from com.atproto.label.defs
	LabelHide:              true,
	LabelNoPromote:         true,
	LabelWarn:              true,
	LabelNoUnauthenticated: true,
	LabelDMCAViolation:     true,
	LabelDoxxing:           true,
	LabelPorn:              true,
	LabelSexual:            true,
	LabelNudity:            true,
	LabelNSFL:              true,
	LabelGore:              true,
	LabelTakedown:          true,
	LabelSuspend:           true,
	LabelSexualFigurative:  true,
	LabelGraphicMedia:      true,
	LabelSelfHarm:          true,
	LabelSensitive:         true,
	LabelExtremist:         true,
	LabelIntolerant:        true,
	LabelThreat:            true,
	LabelRude:              true,
	LabelIllicit:           true,
	LabelSecurity:          true,
	LabelUnsafeLink:        true,
	LabelImpersonation:     true,
	LabelMisinformation:    true,
	LabelScam:              true,
	LabelEngagementFarming: true,
	LabelSpam:              true,
	LabelRumor:             true,
	LabelMisleading:        true,
	LabelInauthentic:       true,
}

// Given a users' labels, determine if they are banned
func IsChatHidden(labels ...*comatproto.LabelDefs_Label) bool {
	for _, l := range labels {
		if !strings.HasPrefix(l.Uri, "did:") {
			// this is a label on a record, not a user
			continue
		}
		if bannedLabels[l.Val] {
			return true
		}
	}
	return false
}
