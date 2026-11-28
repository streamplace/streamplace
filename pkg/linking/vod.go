package linking

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"golang.org/x/net/html"
)

type VODBootPayload struct {
	Manifest any
	Starter  *VODStarter
}

type VODStarter struct {
	Video VODStarterVideo  `json:"video"`
	Audio *VODStarterAudio `json:"audio,omitempty"`
}

type VODStarterVideo struct {
	Track string  `json:"track"`
	Key   *string `json:"key"`
	Rest  *string `json:"rest"`
}

type VODStarterAudio struct {
	Track string  `json:"track"`
	Data  *string `json:"data"`
}

type vodBootScripts struct {
	Boot   string `json:"boot"`
	Engine string `json:"engine"`
}

func (l *Linker) LoadVODBoot(frontend fs.FS) error {
	data, err := fs.ReadFile(frontend, "_expo/static/js/web/vod-boot.json")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("reading VOD boot scripts: %w", err)
	}
	var scripts vodBootScripts
	if err := json.Unmarshal(data, &scripts); err != nil {
		return fmt.Errorf("decoding VOD boot scripts: %w", err)
	}
	if scripts.Boot == "" || scripts.Engine == "" {
		return fmt.Errorf("VOD boot scripts missing boot or engine")
	}
	if strings.Contains(strings.ToLower(scripts.Boot), "</script") {
		return fmt.Errorf("VOD boot script contains closing script tag")
	}
	l.vodBoot = &scripts
	return nil
}

func (l *Linker) HasVODBoot() bool { return l.vodBoot != nil }

func (l *Linker) injectVODBoot(body *html.Node, payload *VODBootPayload) error {
	if payload == nil || l.vodBoot == nil {
		return nil
	}
	if body == nil {
		return fmt.Errorf("VOD boot body not found")
	}
	anchor := body.FirstChild
	insert := func(node *html.Node) { body.InsertBefore(node, anchor) }
	wrapper := &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{{Key: "id", Val: "sp-vod-boot"}, {Key: "data-engine", Val: l.vodBoot.Engine}}}
	wrapper.AppendChild(&html.Node{Type: html.ElementNode, Data: "video", Attr: []html.Attribute{{Key: "playsinline"}}})
	insert(wrapper)
	script := func(id, typ, source string) {
		node := &html.Node{Type: html.ElementNode, Data: "script"}
		if id != "" {
			node.Attr = append(node.Attr, html.Attribute{Key: "id", Val: id}, html.Attribute{Key: "type", Val: typ})
		}
		node.AppendChild(&html.Node{Type: html.TextNode, Data: source})
		insert(node)
	}
	manifest, err := json.Marshal(payload.Manifest)
	if err != nil {
		return fmt.Errorf("encoding VOD manifest: %w", err)
	}
	script("sp-vod-manifest", "application/json", string(manifest))
	if payload.Starter != nil {
		starter, err := json.Marshal(payload.Starter)
		if err != nil {
			return fmt.Errorf("encoding VOD starter: %w", err)
		}
		script("sp-vod-starter", "application/json", string(starter))
	}
	script("", "", l.vodBoot.Boot)
	return nil
}
