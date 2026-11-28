package vod

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	glex "github.com/streamplace/glex/runtime"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/bdasl"
	"stream.place/streamplace/pkg/blob"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

const (
	stationOriginInterval = 30 * time.Second
	stationOriginDeadline = 15 * time.Second
)

// RunStationOrigins publishes this station's durable origin intents. Run it on
// only one explicitly enabled publisher for a station and its playback store.
func RunStationOrigins(ctx context.Context, state *statedb.StatefulDB, store blob.Store) {
	if state == nil || state.CLI == nil || state.CLI.StationDID == "" || store == nil {
		return
	}
	did := state.CLI.StationDID
	var cursor string
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		var client XRPCClient
		var clientUntil time.Time
		getClient := func() (XRPCClient, error) {
			if client != nil && time.Now().Before(clientUntil) {
				return client, nil
			}
			lookupCtx, cancel := context.WithTimeout(ctx, stationOriginDeadline)
			defer cancel()
			station, err := atproto.ResolveStationIdentity(lookupCtx, state.CLI.PLCURL, did)
			if err != nil || station.PDS == "" || station.URL == "" {
				return nil, errors.New("could not resolve station PDS and playback origins")
			}
			client, err = state.StationRepoClient(lookupCtx, did, station.PDS, constants.PLACE_STREAM_MEDIA_ORIGIN, "create", "update", "delete")
			clientUntil = time.Now().Add(stationOriginInterval)
			return client, err
		}
		err := reconcileStationOrigins(ctx, store, did, getClient, func(origin stationOrigin) (bool, error) {
			for _, uri := range origin.sources {
				blocked, err := stationOriginModerated(state.StationOriginLabels, uri)
				if err != nil || !blocked {
					return blocked, err
				}
			}
			return true, nil
		})
		if err == nil {
			var client XRPCClient
			if client, err = getClient(); err == nil {
				cursor, err = repairStationWithdrawals(ctx, store, client, did, cursor)
			}
		}
		if err != nil && ctx.Err() == nil {
			log.Warn(ctx, "station origin reconciliation deferred", "did", did, "error", err)
		}
		timer.Reset(stationOriginInterval)
	}
}

func reconcileStationOrigins(ctx context.Context, store blob.Store, did string, getClient func() (XRPCClient, error), moderated func(stationOrigin) (bool, error)) error {
	prefix := stationOriginPrefix(did)
	listCtx, cancel := context.WithTimeout(ctx, stationOriginDeadline)
	keys, err := store.List(listCtx, prefix)
	cancel()
	if err != nil {
		return errors.New("could not list station origin markers")
	}
	generations := make(map[string]int)
	eventCounts := make(map[string]int)
	markers := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		markers[key] = struct{}{}
		if event, ok := strings.CutPrefix(key, prefix+"video-events/"); ok {
			hash, _, _ := strings.Cut(event, "/")
			eventCounts[hash]++
		} else if hash, ok := strings.CutPrefix(key, prefix+"deleted-video/"); ok {
			eventCounts[hash]++
		}
	}
	for _, key := range keys {
		if source, ok := strings.CutPrefix(key, prefix+"sources/"); ok {
			cid, marker, _ := strings.Cut(source, "/")
			generations[cid]++
			if approval, ok := strings.CutPrefix(marker, "approved/"); ok {
				hash, _, _ := strings.Cut(approval, ".")
				generations[cid] += eventCounts[hash]
			}
		}
	}

	var client XRPCClient
	var clientUntil time.Time
	for _, kind := range []string{"withdrawn/", "public/"} {
		intentPrefix, ackPrefix := prefix+kind, prefix+"acked/"+kind
		for _, key := range keys {
			if err := ctx.Err(); err != nil {
				return err
			}
			cid, ok := strings.CutPrefix(key, intentPrefix)
			if !ok || cid == "" {
				continue
			}
			suffix := cid
			if kind == "public/" && generations[cid] != 0 {
				suffix += "." + strconv.Itoa(generations[cid])
				if _, acked := markers[prefix+"acked/withdrawn/"+suffix]; acked {
					continue
				}
			}
			if _, acked := markers[ackPrefix+suffix]; acked {
				continue
			}
			if _, withdrawn := markers[prefix+"withdrawn/"+cid]; kind == "public/" && withdrawn {
				continue
			}
			if _, err := bdasl.Parse(cid); err != nil {
				continue
			}

			readCtx, cancel := context.WithTimeout(ctx, stationOriginDeadline)
			origin, known, err := stationOriginState(readCtx, store, did, cid)
			cancel()
			if err != nil {
				log.Warn(ctx, "could not read station origin intent", "did", did, "cid", cid)
				continue
			}
			if !known {
				continue
			}
			if !origin.Withdrawn {
				if origin.CID != cid || origin.Size <= 0 {
					log.Warn(ctx, "invalid public station origin intent", "did", did, "cid", cid)
					continue
				}
				blocked, err := moderated(origin)
				if err != nil {
					log.Warn(ctx, "could not check station origin moderation", "did", did, "cid", cid)
					continue
				}
				if blocked {
					continue
				}
			}
			// Long passes recheck credentials without querying sessions per asset.
			if client == nil || !time.Now().Before(clientUntil) {
				client, err = getClient()
				if err != nil {
					return err
				}
				clientUntil = time.Now().Add(stationOriginInterval)
			}
			recordCtx, cancel := context.WithTimeout(ctx, stationOriginDeadline)
			err = reconcileStationOrigin(recordCtx, store, client, did, cid, origin)
			cancel()
			if err != nil && ctx.Err() == nil {
				log.Warn(ctx, "station origin pending", "did", did, "cid", cid, "error", err)
			}
		}
	}
	return nil
}

func stationOriginModerated(labels func(string) ([]*comatproto.LabelDefs_Label, error), uri string) (bool, error) {
	if uri == "" {
		return false, nil
	}
	video, err := parseVideoURI(uri)
	if err != nil {
		return false, err
	}
	did, err := video.Authority().AsDID()
	if err != nil {
		return false, err
	}
	recordLabels, err := labels(video.String())
	if err != nil || len(recordLabels) > 0 {
		return len(recordLabels) > 0, err
	}
	accountLabels, err := labels(did.String())
	return atproto.IsBanned(accountLabels...), err
}

func reconcileStationOrigin(ctx context.Context, store blob.Store, client XRPCClient, did, cid string, origin stationOrigin) error {
	collection := constants.PLACE_STREAM_MEDIA_ORIGIN
	prefix := stationOriginPrefix(did)
	ack := prefix + "acked/withdrawn/" + cid
	var desired *placestream.MediaOrigin
	if !origin.Withdrawn {
		ack = prefix + "acked/public/" + cid
		desired = &placestream.MediaOrigin{
			LexiconTypeID: constants.PLACE_STREAM_MEDIA_ORIGIN,
			Blob:          cid,
			Size:          origin.Size,
			MimeType:      transferMimeType,
		}
	}
	if origin.generation != 0 {
		ack += "." + strconv.Itoa(origin.generation)
	}

	// A missing ACK also covers process loss after a successful PDS write.
	matches, err := stationOriginMatches(ctx, client, did, cid, desired)
	if err != nil {
		return err
	}
	if !matches {
		if desired == nil {
			input := &comatproto.RepoDeleteRecord_Input{Repo: did, Collection: collection, Rkey: cid}
			if !origin.terminal {
				// Journal the exact parent before dispatch, including ambiguous
				// responses. A delayed echo must not turn this into a CID ban.
				var head comatproto.SyncGetLatestCommit_Output
				if err := client.Do(ctx, xrpc.Query, "", "com.atproto.sync.getLatestCommit", map[string]any{"did": did}, nil, &head); err != nil {
					return err
				}
				if _, err := syntax.ParseTID(head.Rev); err != nil || head.Cid == "" {
					return errors.New("station PDS returned an invalid repo head")
				}
				current, known, err := stationOriginState(ctx, store, did, cid)
				if err != nil || !known || !current.Withdrawn {
					return err
				}
				if !current.terminal {
					if err := writeStationMarker(ctx, store, prefix+"source-origin-delete/"+cid+"/"+head.Rev, nil); err != nil {
						return err
					}
					input.SwapCommit = &head.Cid
				}
			}
			err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.deleteRecord", nil, input, nil)
		} else {
			// Never send a put if a withdrawal appeared during preflight.
			withdrawn, markerErr := stationMarkerExists(ctx, store, prefix+"withdrawn/"+cid)
			if markerErr != nil {
				return errors.New("could not check station origin withdrawal")
			}
			if withdrawn {
				return nil
			}
			body := &comatproto.RepoPutRecord_Input{
				Repo: did, Collection: collection, Rkey: cid,
				Record: &glex.LexiconTypeDecoder{Val: desired},
			}
			var out comatproto.RepoPutRecord_Output
			err = client.Do(ctx, xrpc.Procedure, "application/json", "com.atproto.repo.putRecord", nil, body, &out)
			if err == nil && (out.Uri != "at://"+did+"/"+collection+"/"+cid || out.Cid == "") {
				err = errors.New("station PDS returned an invalid put response")
			}
		}
		if err != nil {
			return verifyStationOriginWrite(ctx, store, client, did, cid, ack, desired, err)
		}
	}
	return ackStationOrigin(ctx, store, ack)
}

func verifyStationOriginWrite(ctx context.Context, store blob.Store, client XRPCClient, did, cid, ack string, desired *placestream.MediaOrigin, writeErr error) error {
	var xerr *xrpc.Error
	var herr *echo.HTTPError
	if (errors.As(writeErr, &xerr) && xerr.StatusCode >= 400 && xerr.StatusCode < 500 && xerr.StatusCode != http.StatusRequestTimeout) ||
		(errors.As(writeErr, &herr) && herr.Code == http.StatusTooManyRequests) {
		return errors.New("station PDS rejected origin write")
	}
	if ctx.Err() == nil {
		if matches, err := stationOriginMatches(ctx, client, did, cid, desired); err == nil && matches {
			return ackStationOrigin(ctx, store, ack)
		}
	}
	return errors.New("station origin write not confirmed")
}

func ackStationOrigin(ctx context.Context, store blob.Store, key string) error {
	if err := writeStationMarker(ctx, store, key, []byte("{}")); err != nil {
		return errors.New("could not persist station origin ACK")
	}
	return nil
}

func stationOriginMatches(ctx context.Context, client XRPCClient, did, cid string, desired *placestream.MediaOrigin) (bool, error) {
	collection := constants.PLACE_STREAM_MEDIA_ORIGIN
	var out comatproto.RepoGetRecord_Output
	err := client.Do(ctx, xrpc.Query, "", "com.atproto.repo.getRecord", map[string]any{
		"repo": did, "collection": collection, "rkey": cid,
	}, nil, &out)
	if err != nil {
		var response *xrpc.Error
		var xerr *xrpc.XRPCError
		if errors.As(err, &response) && (response.StatusCode == http.StatusBadRequest || response.StatusCode == http.StatusNotFound) &&
			errors.As(err, &xerr) && xerr.ErrStr == "RecordNotFound" {
			return desired == nil, nil
		}
		return false, errors.New("could not read origin record from station PDS")
	}
	if out.Uri != "at://"+did+"/"+collection+"/"+cid || out.Value == nil {
		return false, errors.New("station PDS returned an invalid origin response")
	}
	if desired == nil {
		return false, nil
	}
	got, ok := out.Value.Val.(*placestream.MediaOrigin)
	return ok && got != nil && *got == *desired, nil
}

// A timed-out former publisher may have a PUT in flight while a replacement
// deletes and ACKs the same CID. Check one real PDS page per pass, including
// already-ACKed withdrawals. A retry or cache flush alone cannot fix this.
func repairStationWithdrawals(ctx context.Context, store blob.Store, client XRPCClient, did, cursor string) (string, error) {
	params := map[string]any{"repo": did, "collection": constants.PLACE_STREAM_MEDIA_ORIGIN, "limit": 100}
	if cursor != "" {
		params["cursor"] = cursor
	}
	var page comatproto.RepoListRecords_Output
	pageCtx, cancel := context.WithTimeout(ctx, stationOriginDeadline)
	err := client.Do(pageCtx, xrpc.Query, "", "com.atproto.repo.listRecords", params, nil, &page)
	cancel()
	if err != nil {
		return cursor, errors.New("could not audit station PDS origins")
	}
	for _, record := range page.Records {
		uri, err := syntax.ParseATURI(record.Uri)
		if err != nil || uri.Authority().String() != did || uri.Collection().String() != constants.PLACE_STREAM_MEDIA_ORIGIN {
			continue
		}
		cid := uri.RecordKey().String()
		rowCtx, cancel := context.WithTimeout(ctx, stationOriginDeadline)
		intent, known, err := stationOriginState(rowCtx, store, did, cid)
		if err == nil && known && intent.Withdrawn {
			err = reconcileStationOrigin(rowCtx, store, client, did, cid, intent)
		}
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return cursor, ctx.Err()
			}
			log.Warn(ctx, "could not reconcile audited station origin", "did", did, "cid", cid)
		}
	}
	if page.Cursor == nil || *page.Cursor == cursor {
		return "", nil
	}
	return *page.Cursor, nil
}
