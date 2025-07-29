package atproto

import (
	"strings"

	comatproto "github.com/bluesky-social/indigo/api/atproto"
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
)

var bannedLabels = map[string]bool{
	LabelDMCAViolation: true,
	LabelDoxxing:       true,
	LabelPorn:          true,
	LabelSexual:        true,
	LabelNudity:        true,
	LabelNSFL:          true,
	LabelGore:          true,
	LabelTakedown:      true,
	LabelSuspend:       true,
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
