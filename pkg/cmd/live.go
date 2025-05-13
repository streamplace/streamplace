package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func Live(streamKey string) error {
	// Create the URL for the live stream endpoint
	url := fmt.Sprintf("http://127.0.0.1:39090/live/%s", streamKey)

	// Create a new HTTP request with POST method
	req, err := http.NewRequest("POST", url, os.Stdin)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	// Set appropriate headers if needed
	req.Header.Set("Content-Type", "video/x-matroska") // Assuming MKV format, adjust if needed

	// Create HTTP client and send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending stream: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned non-OK status: %d %s - %s",
			resp.StatusCode, resp.Status, string(body))
	}

	// Copy response to stdout (if any)
	_, err = io.Copy(os.Stdout, resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}

	return nil
}
