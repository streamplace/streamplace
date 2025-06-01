package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/net/context/ctxhttp"
	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/streamplace"
)

type Webhook struct {
	DID  string `json:"did"`
	URL  string `json:"url"`
	Type string `json:"type"`
}

type Payload struct {
	Username  string  `json:"username,omitempty"`
	AvatarURL string  `json:"avatar_url,omitempty"`
	Content   string  `json:"content,omitempty"`
	Embeds    []Embed `json:"embeds,omitempty"`
}

type Embed struct {
	Author      Author  `json:"author,omitempty"`
	Title       string  `json:"title,omitempty"`
	URL         string  `json:"url,omitempty"`
	Description string  `json:"description,omitempty"`
	Color       int     `json:"color,omitempty"`
	Fields      []Field `json:"fields,omitempty"`
	Thumbnail   Image   `json:"thumbnail,omitempty"`
	Image       Image   `json:"image,omitempty"`
	Footer      Footer  `json:"footer,omitempty"`
}

type Author struct {
	Name    string `json:"name,omitempty"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

type Field struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

type Image struct {
	URL string `json:"url,omitempty"`
}

type Footer struct {
	Text    string `json:"text,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

func (w *Webhook) SendLivestream(ctx context.Context, lsv *streamplace.Livestream_LivestreamView) error {
	ls, ok := lsv.Record.Val.(*streamplace.Livestream)
	if !ok {
		return fmt.Errorf("failed to cast livestream view to livestream")
	}
	payload := Payload{
		Username: "Streamplace",
		Content:  fmt.Sprintf("New livestream from %s: %s", w.DID, ls.Title),
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", w.URL, bytes.NewReader(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ctxhttp.Do(ctx, &aqhttp.Client, req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
