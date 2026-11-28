package linking

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestVODBootInjection(t *testing.T) {
	linker, err := NewLinker(context.Background(), []byte(`<html><head></head><body><div id="root"></div></body></html>`), nil, nil)
	require.NoError(t, err)
	payload := &VODBootPayload{Manifest: map[string]string{"uri": "</script><script>alert('x')</script>&\u2028"}, Starter: &VODStarter{Video: VODStarterVideo{Track: "1"}}}
	pc := &PageConfig{VODBoot: payload}
	baseline, err := linker.GenerateHTML(context.Background(), pc)
	require.NoError(t, err)
	require.NotContains(t, string(baseline), "sp-vod-")
	require.NoError(t, linker.LoadVODBoot(fstest.MapFS{}))
	absent, err := linker.GenerateHTML(context.Background(), pc)
	require.NoError(t, err)
	require.Equal(t, baseline, absent)
	scripts, err := json.Marshal(vodBootScripts{Boot: "window.booted=true;", Engine: "/_expo/static/js/web/vod-engine-test.js"})
	require.NoError(t, err)
	require.NoError(t, linker.LoadVODBoot(fstest.MapFS{"_expo/static/js/web/vod-boot.json": &fstest.MapFile{Data: scripts}}))
	require.True(t, linker.HasVODBoot())
	out, err := linker.GenerateHTML(context.Background(), pc)
	require.NoError(t, err)
	page := string(out)
	require.Contains(t, page, `<body><div id="sp-vod-boot" data-engine="/_expo/static/js/web/vod-engine-test.js"><video playsinline=""></video></div>`)
	require.Contains(t, page, `<script id="sp-vod-manifest" type="application/json">`)
	require.Contains(t, page, `\u003c/script\u003e`)
	require.NotContains(t, page, `<script>alert`)
	require.Contains(t, page, `\u0026\u2028`)
	require.Contains(t, page, `<script id="sp-vod-starter" type="application/json">{"video":{"track":"1","key":null,"rest":null}}</script>`)
	require.Less(t, strings.Index(page, "window.booted=true;"), strings.Index(page, `id="root"`))
	payload.Starter = nil
	out, err = linker.GenerateHTML(context.Background(), pc)
	require.NoError(t, err)
	require.NotContains(t, string(out), "sp-vod-starter")
	pc.VODBoot = nil
	out, err = linker.GenerateHTML(context.Background(), pc)
	require.NoError(t, err)
	require.NotContains(t, string(out), "sp-vod-")
}
