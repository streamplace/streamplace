package oproxy

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func boolPtr(b bool) *bool {
	return &b
}

func codeUUID(prefix string) string {
	uu, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s-%s", prefix, uu.String())
}

var urnPrefix = "urn:ietf:params:oauth:request_uri:"

const UUID_LENGTH = 37

func makeURN(jkt string) string {
	uu, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s%s-%s", urnPrefix, uu.String(), jkt)
}

// urn --> jkt, uu
func parseURN(urn string) (string, string, error) {
	if !strings.HasPrefix(urn, urnPrefix) {
		return "", "", fmt.Errorf("invalid URN: %s", urn)
	}
	withoutPrefix := urn[len(urnPrefix):]
	uu := withoutPrefix[:UUID_LENGTH]
	suffix := withoutPrefix[UUID_LENGTH:]
	return suffix, uu, nil
}

func makeState(jkt string) string {
	uu, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s-%s", uu.String(), jkt)
}

func parseState(state string) (string, string, error) {
	if len(state) < UUID_LENGTH {
		return "", "", fmt.Errorf("invalid state: %s", state)
	}
	uu := state[:UUID_LENGTH]
	suffix := state[UUID_LENGTH:]
	return suffix, uu, nil
}
