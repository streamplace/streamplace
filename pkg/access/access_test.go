package access

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGrantURIRoundTrip(t *testing.T) {
	uri := GrantURI("did:web:node.example", "did:plc:admin", "3abc")
	require.Equal(t, "at://did:web:node.example/space/place.stream.access.control/self/did:plc:admin/place.stream.access.grant/3abc", uri)
	ref, err := ParseGrantURI(uri)
	require.NoError(t, err)
	require.Equal(t, GrantRef{Authority: "did:web:node.example", Author: "did:plc:admin", RKey: "3abc"}, ref)
}

func TestParseGrantURIRejectsOtherShapes(t *testing.T) {
	for _, bad := range []string{
		"",
		"at://did:web:x/place.stream.access.grant/3abc",                                                   // plain repo record
		"ats://did:web:x/space/place.stream.access.control/self/did:plc:a/place.stream.access.grant/3abc", // old scheme
		SpaceURI("did:web:x"),  // the space itself
		PolicyURI("did:web:x"), // the policy record
		"at://did:web:x/space/place.stream.other/self/did:plc:a/place.stream.access.grant/3abc", // other space type
	} {
		_, err := ParseGrantURI(bad)
		require.Error(t, err, bad)
	}
}

func TestValidRoleAndMode(t *testing.T) {
	for _, r := range Roles {
		require.True(t, ValidRole(r))
	}
	require.False(t, ValidRole("superuser"))
	require.True(t, ValidMode(ModeOpen))
	require.False(t, ValidMode("closed"))
}

func TestParsePolicy(t *testing.T) {
	p, err := ParsePolicy("viewer=allowlist, vod=off,")
	require.NoError(t, err)
	require.Equal(t, map[string]string{RoleViewer: ModeAllowlist, RoleVOD: ModeOff}, p)
	for _, bad := range []string{"viewer", "viewer=sometimes", "wizard=open", "admin=open"} {
		_, err := ParsePolicy(bad)
		require.Error(t, err, bad)
	}
	p, err = ParsePolicy("")
	require.NoError(t, err)
	require.Empty(t, p)
}
