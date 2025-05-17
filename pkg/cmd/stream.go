package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func Stream(u string) error {
	resp, err := http.Get(u)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("http status %s", resp.Status)
	}
	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		return fmt.Errorf("failed copying body: %w", err)
	}
	return nil
}
