package cmd

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	urfavecli "github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"
	"moq.dev/moq"
	"stream.place/streamplace/pkg/log"
)

// A proof of concept for VOD time to first frame; see js/vod-poc/README.md.

type pocFrame struct {
	TS   uint64 `json:"ts"`   // microseconds
	Data []byte `json:"data"` // AVCC
}

type pocRendition struct {
	Name        string `json:"name"`
	File        string `json:"file,omitempty"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FPS         int    `json:"fps"`
	Codec       string `json:"codec"`
	Description []byte `json:"description"`
	Groups      int    `json:"groups"`
	groups      [][]pocFrame
}

func makeVodPocCommand() *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "vod-poc",
		Usage: "serve the VOD time-to-first-frame proof of concept (js/vod-poc)",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{Name: "media", Usage: "directory written by js/vod-poc/prepare-media.sh", Required: true},
			&urfavecli.StringFlag{Name: "web", Usage: "the js/vod-poc directory, built", Value: "js/vod-poc"},
			&urfavecli.StringFlag{Name: "http-addr", Value: "127.0.0.1:0"},
			&urfavecli.StringFlag{Name: "moq-addr", Value: "127.0.0.1:0"},
		},
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			return runVodPoc(ctx, cmd.String("media"), cmd.String("web"), cmd.String("http-addr"), cmd.String("moq-addr"))
		},
	}
}

func runVodPoc(ctx context.Context, mediaDir, webDir, httpAddr, moqAddr string) error {
	renditions, err := loadPocRenditions(mediaDir)
	if err != nil {
		return err
	}
	srv, err := moq.Listen(ctx, moqAddr, moq.WithTLSGenerate("localhost", "127.0.0.1"))
	if err != nil {
		return fmt.Errorf("listening for moq: %w", err)
	}
	defer srv.Close()
	fingerprints, err := srv.CertFingerprints()
	if err != nil || len(fingerprints) == 0 {
		return fmt.Errorf("moq certificate fingerprint: %w", err)
	}

	tmpl, err := template.ParseFiles(filepath.Join(webDir, "index.html"))
	if err != nil {
		return err
	}
	player, err := os.ReadFile(filepath.Join(webDir, "dist", "player.js"))
	if err != nil {
		return fmt.Errorf("reading the player bundle (build js/vod-poc first): %w", err)
	}
	playerSum := sha256.Sum256(player)
	playerURL := "/vod-poc/player.js?v=" + hex.EncodeToString(playerSum[:6])

	httpLn, err := net.Listen("tcp", httpAddr)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.Handle("GET /vod-poc/player.js", newPocAsset("text/javascript; charset=utf-8", "public, max-age=31536000, immutable", player))
	var urls []string
	var producers []any
	defer func() { runtime.KeepAlive(producers) }()
	for _, tid := range []string{"one", "two"} {
		path := "vod-poc/" + tid
		published, err := publishPocBroadcast(ctx, srv, path, renditions)
		if err != nil {
			return err
		}
		producers = append(producers, published...)
		html, err := bakePocPage(tmpl, tid, renditions, map[string]string{
			"url":         "https://" + srv.LocalAddr(),
			"fingerprint": fingerprints[0],
			"path":        path,
		}, playerURL)
		if err != nil {
			return err
		}
		page := newPocAsset("text/html; charset=utf-8", "no-cache", html)
		mux.Handle("GET /vod-poc/video/"+tid, page)
		urls = append(urls, "http://"+httpLn.Addr().String()+"/vod-poc/video/"+tid)
		log.Log(ctx, "baked vod-poc page", "tid", tid, "bytes", len(page.raw), "gzip", len(page.gz))
	}

	// Scripts read this line for the two watch URLs.
	fmt.Printf("VOD_POC_URLS=%s\n", strings.Join(urls, ","))
	httpSrv := &http.Server{Handler: mux}
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return srv.Serve(gctx) })
	g.Go(func() error {
		<-gctx.Done()
		return httpSrv.Close()
	})
	g.Go(func() error {
		if err := httpSrv.Serve(httpLn); !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})
	return g.Wait()
}

// pocAsset is a response body, kept plain and gzipped.
type pocAsset struct {
	contentType, cacheControl string
	raw, gz                   []byte
}

func newPocAsset(contentType, cacheControl string, raw []byte) pocAsset {
	var gz bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&gz, gzip.BestCompression)
	_, _ = zw.Write(raw)
	_ = zw.Close()
	return pocAsset{contentType: contentType, cacheControl: cacheControl, raw: raw, gz: gz.Bytes()}
}

func (a pocAsset) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", a.contentType)
	w.Header().Set("Cache-Control", a.cacheControl)
	body := a.raw
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		body = a.gz
	}
	_, _ = w.Write(body)
}

// bakePocPage renders a watch page carrying the manifest and the starter's
// first group: its keyframe in one script, the rest of the group in the next.
func bakePocPage(tmpl *template.Template, tid string, renditions []*pocRendition, moqInfo map[string]string, playerURL string) ([]byte, error) {
	starter := renditions[0].groups[0]
	marshal := func(v any) string {
		b, _ := json.Marshal(v)
		return string(b)
	}
	var html bytes.Buffer
	err := tmpl.Execute(&html, map[string]string{
		"Title": "Proof of concept video " + tid,
		"Manifest": marshal(map[string]any{
			"moq":        moqInfo,
			"gopUs":      1_000_000,
			"renditions": renditions,
		}),
		"Keyframe": marshal(starter[:1]),
		"Rest":     marshal(starter[1:]),
		"Player":   playerURL,
	})
	return html.Bytes(), err
}

// publishPocBroadcast publishes each rendition as a finished track and serves
// every fetched group on demand, so none ages out of the library's idle cache.
// The broadcast ends when the producers it returns are dropped.
func publishPocBroadcast(ctx context.Context, srv *moq.Server, path string, renditions []*pocRendition) ([]any, error) {
	bc, err := srv.CreateBroadcast(path)
	if err != nil {
		return nil, err
	}
	producers := []any{bc}
	forever := uint64(1<<62 - 1)
	for _, r := range renditions {
		track, err := bc.PublishTrack(r.Name, &moq.TrackInfo{Priority: 127, MaxAgeUs: &forever})
		if err != nil {
			return nil, err
		}
		dynamic, err := track.Dynamic()
		if err != nil {
			return nil, err
		}
		if err := track.FinishAt(uint64(len(r.groups))); err != nil {
			return nil, err
		}
		producers = append(producers, track, dynamic)
		go func() {
			for req, err := range dynamic.Requests(ctx) {
				if err != nil {
					return
				}
				if err := servePocGroup(req, r.groups); err != nil {
					log.Warn(ctx, "serving vod-poc group", "path", path, "track", r.Name, "error", err)
				}
			}
		}()
	}
	return producers, bc.Announce(moq.Route{})
}

func servePocGroup(req *moq.GroupRequest, groups [][]pocFrame) error {
	seq := req.Sequence()
	if seq >= uint64(len(groups)) {
		return req.Abort(404)
	}
	group, err := req.Accept()
	if err != nil {
		return err
	}
	for _, f := range groups[seq] {
		if err := group.WriteFrame(moq.Frame{Payload: f.Data, TimestampUs: f.TS}); err != nil {
			return err
		}
	}
	return group.Finish()
}

func loadPocRenditions(dir string) ([]*pocRendition, error) {
	b, err := os.ReadFile(filepath.Join(dir, "renditions.json"))
	if err != nil {
		return nil, err
	}
	var renditions []*pocRendition
	if err := json.Unmarshal(b, &renditions); err != nil {
		return nil, err
	}
	for _, r := range renditions {
		annexB, err := os.ReadFile(filepath.Join(dir, r.File))
		if err != nil {
			return nil, err
		}
		if err := r.parse(annexB); err != nil {
			return nil, fmt.Errorf("%s: %w", r.File, err)
		}
		r.File = ""
	}
	if len(renditions) == 0 || len(renditions[0].groups) == 0 {
		return nil, errors.New("no media")
	}
	return renditions, nil
}

// parse splits an Annex B stream with an access unit delimiter per frame
// into keyframe-led groups of AVCC frames, and builds the avcC description.
func (r *pocRendition) parse(annexB []byte) error {
	var sps, pps []byte
	var frame []byte
	key := false
	n := 0
	flush := func() {
		if frame == nil {
			return
		}
		if key || len(r.groups) == 0 {
			r.groups = append(r.groups, nil)
		}
		last := len(r.groups) - 1
		r.groups[last] = append(r.groups[last], pocFrame{TS: uint64(n) * 1_000_000 / uint64(r.FPS), Data: frame})
		frame, key = nil, false
		n++
	}
	for _, nal := range splitAnnexB(annexB) {
		switch nal[0] & 0x1f {
		case 9: // access unit delimiter
			flush()
		case 7:
			sps = nal
		case 8:
			pps = nal
		case 6: // SEI
		default:
			key = key || nal[0]&0x1f == 5
			frame = binary.BigEndian.AppendUint32(frame, uint32(len(nal)))
			frame = append(frame, nal...)
		}
	}
	flush()
	if sps == nil || pps == nil {
		return errors.New("no SPS/PPS")
	}
	r.Codec = fmt.Sprintf("avc1.%02x%02x%02x", sps[1], sps[2], sps[3])
	desc := []byte{1, sps[1], sps[2], sps[3], 0xff, 0xe1}
	desc = binary.BigEndian.AppendUint16(desc, uint16(len(sps)))
	desc = append(desc, sps...)
	desc = append(desc, 1)
	desc = binary.BigEndian.AppendUint16(desc, uint16(len(pps)))
	r.Description = append(desc, pps...)
	r.Groups = len(r.groups)
	return nil
}

// splitAnnexB returns the NAL units between start codes, without trailing zeros.
func splitAnnexB(b []byte) [][]byte {
	var nals [][]byte
	start := -1
	for i := 0; i+2 < len(b); i++ {
		if b[i] == 0 && b[i+1] == 0 && b[i+2] == 1 {
			if start >= 0 {
				nals = append(nals, bytes.TrimRight(b[start:i], "\x00"))
			}
			start = i + 3
			i += 2
		}
	}
	if start >= 0 && start < len(b) {
		nals = append(nals, b[start:])
	}
	return nals
}
