package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func Live(streamKey string, httpInternalAddr string) error {
	// Live POSTs a fragmented-MP4 stream from stdin to the node's /live ingest
	// route. (The Mist ingest bridge no longer uses this — the node pulls Mist's
	// fMP4 output directly; see MistPullIngest.)
	url := fmt.Sprintf("http://%s/live/%s", httpInternalAddr, streamKey)

	// Create a new HTTP request with POST method
	req, err := http.NewRequest("POST", url, os.Stdin)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	// Set appropriate headers if needed
	req.Header.Set("Content-Type", "video/mp4") // fragmented MP4 from stdin

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
