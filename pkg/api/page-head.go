package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	apierrors "stream.place/streamplace/pkg/errors"
	"stream.place/streamplace/pkg/linking"
	"stream.place/streamplace/pkg/log"
)

// cardHead builds a page's own head, false when the page has no card of its
// own (or its card can't be built) and the default card applies.
func (a *StreamplaceAPI) cardHead(ctx context.Context, linker *linking.Linker, u *url.URL) ([]byte, bool) {
	// The app's tabs each get their own card: the front page (live),
	// /video (on demand) and /live (go live).
	if page := linking.LandingPageFor(u.Path); page != linking.LandingDefault {
		head, err := linker.GenerateLandingCard(ctx, u, page, a.CLI.SentryDSN)
		if err != nil {
			log.Error(ctx, "error generating landing card", "path", u.Path, "error", err)
			return nil, false
		}
		return head, true
	}

	// VOD link cards live at /<user>/video/<tid>.
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) == 3 && parts[1] == "video" {
		return a.videoCardHead(ctx, linker, u, parts[0], parts[2])
	}

	// A single path segment is a streamer handle or DID. The worker URL is
	// reserved; asset-like TLDs can still be handles.
	handle := strings.TrimPrefix(u.Path, "/")
	if !strings.ContainsAny(handle, "/_") && handle != "sw.js" {
		return a.streamerCardHead(ctx, linker, u, handle)
	}
	return nil, false
}

// pageHead builds the head for any page: its own card when it has one, the
// node's default card otherwise.
func (a *StreamplaceAPI) pageHead(ctx context.Context, linker *linking.Linker, u *url.URL) ([]byte, error) {
	if head, ok := a.cardHead(ctx, linker, u); ok {
		return head, nil
	}
	return linker.GenerateDefaultCard(ctx, u, a.CLI.SentryDSN)
}

// videoCardHead renders the card for a VOD at /<user>/video/<tid>, false
// when the video can't be found or rendered.
func (a *StreamplaceAPI) videoCardHead(ctx context.Context, linker *linking.Linker, u *url.URL, user, tid string) ([]byte, bool) {
	repo, err := a.Model.GetRepoByHandleOrDID(user)
	if err != nil || repo == nil {
		return nil, false
	}
	uri := fmt.Sprintf("at://%s/place.stream.video/%s", repo.DID, tid)
	vv, err := a.Model.GetVideoView(ctx, uri)
	if err != nil {
		log.Error(ctx, "error fetching video view for card", "uri", uri, "error", err)
		return nil, false
	}
	if vv == nil {
		return nil, false
	}
	a.hydrateAuthorName(ctx, &vv.Author)
	head, err := linker.GenerateVideoCard(ctx, u, vv, a.CLI.SentryDSN)
	if err != nil {
		log.Error(ctx, "error generating video card", "uri", uri, "error", err)
		return nil, false
	}
	return head, true
}

// streamerCardHead renders the card for a streamer page, false when the
// account or its livestream isn't indexed.
func (a *StreamplaceAPI) streamerCardHead(ctx context.Context, linker *linking.Linker, u *url.URL, handle string) ([]byte, bool) {
	repo, err := a.Model.GetRepoByHandleOrDID(handle)
	if err != nil || repo == nil {
		log.Error(ctx, "no repo found", "maybeHandle", handle)
		return nil, false
	}
	ls, err := a.Model.GetLatestLivestreamForRepo(repo.DID)
	if err != nil || ls == nil {
		log.Error(ctx, "no livestream found", "repoDID", repo.DID)
		return nil, false
	}
	lsv, err := ls.ToLivestreamView()
	if err != nil || lsv == nil {
		log.Error(ctx, "no livestream view found", "repoDID", repo.DID)
		return nil, false
	}
	a.hydrateAuthorName(ctx, &lsv.Author)
	head, err := linker.GenerateStreamerCard(ctx, u, lsv, a.CLI.SentryDSN)
	if err != nil {
		log.Error(ctx, "error generating streamer card", "repoDID", repo.DID, "error", err)
		return nil, false
	}
	return head, true
}

// HandlePageHead returns the head a page carries, for the edge worker to
// splice into its own copy of the shell.
func (a *StreamplaceAPI) HandlePageHead(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		raw := req.URL.Query().Get("url")
		u, err := url.Parse(raw)
		if raw == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			apierrors.WriteHTTPBadRequest(w, "url must be an absolute http(s) URL", err)
			return
		}
		head, err := a.pageHead(ctx, a.headLinker, u)
		if err != nil {
			apierrors.WriteHTTPInternalServerError(w, "error generating page head", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(map[string]string{"head": string(head)}); err != nil {
			log.Error(ctx, "error writing page head", "error", err)
		}
	}
}
