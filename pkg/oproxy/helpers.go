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

func makeURN(jkt string) string {
	uu, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s%s_%s", urnPrefix, jkt, uu.String())
}

// urn --> jkt, uu
func parseURN(urn string) (string, string, error) {
	if !strings.HasPrefix(urn, urnPrefix) {
		return "", "", fmt.Errorf("invalid URN: %s", urn)
	}
	parts := strings.Split(urn[len(urnPrefix):], "_")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid URN: %s", urn)
	}
	return parts[0], parts[1], nil
}
