package oproxy

import (
	"fmt"

	"github.com/google/uuid"
)

func generateRefreshToken() (string, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("refresh-%s", uu.String()), nil
}

func generateAuthorizationCode() (string, error) {
	uu, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("code-%s", uu.String()), nil
}
