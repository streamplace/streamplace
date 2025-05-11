package oproxy

import (
	"fmt"

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
