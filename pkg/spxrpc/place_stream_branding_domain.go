package spxrpc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"github.com/labstack/echo/v4"
	glex "github.com/streamplace/glex/runtime"
	"github.com/streamplace/oatproxy/pkg/oatproxy"

	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/atproto"
	"stream.place/streamplace/pkg/branding"
	"stream.place/streamplace/pkg/comatproto"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/placestream"
	"stream.place/streamplace/pkg/statedb"
)

// Custom domains. An admin grants a hostname to an account; the account's
// place.stream.branding.brand record keyed by that hostname is then the
// domain's brand. The node keeps a copy as ordinary branding rows under
// did:web:<hostname> (so every read path is the same as for the node's own
// brand) and refreshes it from the record: on grant, on the owner's edits
// through this node, on firehose events for the record, on syncDomain, and
// periodically (SyncBrandingDomains).

func domainView(d *statedb.BrandingDomain) *placestream.BrandingDefs_DomainView {
	v := &placestream.BrandingDefs_DomainView{
		Hostname: d.Hostname,
		Owner:    d.OwnerDID,
		Brand:    d.BrandID(),
	}
	rec := fmt.Sprintf("at://%s/%s/%s", d.OwnerDID, branding.RecordNSID, d.Hostname)
	v.Record = &rec
	if d.RecordCID != "" {
		c := d.RecordCID
		v.RecordCid = &c
	}
	if d.SyncedAt != nil {
		t := d.SyncedAt.UTC().Format(time.RFC3339)
		v.SyncedAt = &t
	}
	if d.SyncError != "" {
		e := d.SyncError
		v.SyncError = &e
	}
	return v
}

func (s *Server) handlePlaceStreamBrandingPutDomain(ctx context.Context, input *placestream.BrandingPutDomain_Input) (*placestream.BrandingDefs_DomainView, error) {
	caller, err := s.requireAdmin(ctx, "add custom domains")
	if err != nil {
		return nil, err
	}
	host := branding.NormalizeHostname(input.Hostname)
	if host == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidHostname: not a hostname")
	}
	if host == s.cli.BroadcasterHost || host == s.cli.ServerHost {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "InvalidHostname: that is the node's own hostname")
	}
	owner := caller
	if input.Owner != nil && *input.Owner != "" {
		owner = *input.Owner
	}
	unlock := s.lockDomain(host)
	d, err := s.statefulDB.PutBrandingDomain(host, owner)
	unlock()
	if err != nil {
		log.Error(ctx, "failed to add custom domain", "hostname", host, "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "unable to add custom domain")
	}
	log.Log(ctx, "custom domain added", "hostname", host, "owner", owner, "by", caller)
	// A first pull now, so a domain whose owner already published a brand
	// is branded at once; a failure is recorded on the domain, not fatal.
	return domainView(s.SyncBrandingDomain(ctx, d)), nil
}

// domainForWrite finds a custom domain the caller may manage: its owner, or
// an admin.
func (s *Server) domainForWrite(ctx context.Context, hostname string) (*statedb.BrandingDomain, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	d, err := s.statefulDB.GetBrandingDomain(branding.NormalizeHostname(hostname))
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "unable to read custom domains")
	}
	if d == nil || (d.OwnerDID != session.DID && !s.isAdminDID(session.DID)) {
		// Not found and not yours read the same: domains are not listed publicly.
		return nil, echo.NewHTTPError(http.StatusNotFound, "DomainNotFound: no such custom domain")
	}
	return d, nil
}

func (s *Server) handlePlaceStreamBrandingDeleteDomain(ctx context.Context, input *placestream.BrandingDeleteDomain_Input) (*placestream.BrandingDeleteDomain_Output, error) {
	d, err := s.domainForWrite(ctx, input.Hostname)
	if err != nil {
		return nil, err
	}
	unlock := s.lockDomain(d.Hostname)
	err = s.statefulDB.DeleteBrandingDomain(d.Hostname)
	unlock()
	if err != nil {
		log.Error(ctx, "failed to delete custom domain", "hostname", d.Hostname, "err", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "unable to delete custom domain")
	}
	log.Log(ctx, "custom domain removed", "hostname", d.Hostname)
	return &placestream.BrandingDeleteDomain_Output{Success: true}, nil
}

func (s *Server) handlePlaceStreamBrandingListDomains(ctx context.Context) (*placestream.BrandingListDomains_Output, error) {
	session, _ := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	owner := session.DID
	if s.isAdminDID(session.DID) {
		owner = ""
	}
	domains, err := s.statefulDB.ListBrandingDomains(owner)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "unable to list custom domains")
	}
	out := &placestream.BrandingListDomains_Output{Domains: make([]placestream.BrandingDefs_DomainView, 0, len(domains))}
	for i := range domains {
		out.Domains = append(out.Domains, *domainView(&domains[i]))
	}
	return out, nil
}

func (s *Server) handlePlaceStreamBrandingSyncDomain(ctx context.Context, input *placestream.BrandingSyncDomain_Input) (*placestream.BrandingDefs_DomainView, error) {
	d, err := s.domainForWrite(ctx, input.Hostname)
	if err != nil {
		return nil, err
	}
	return domainView(s.SyncBrandingDomain(ctx, d)), nil
}

// lockDomain serializes everything that changes a custom domain's cached
// brand or ownership on this node: syncs, edits, grants and removals.
func (s *Server) lockDomain(hostname string) func() {
	v, _ := s.domainLocks.LoadOrStore(hostname, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// SyncBrandingDomain pulls d's brand record and replaces the domain's
// cached branding with it. An owner with no record gets an unbranded domain
// (the app defaults, never the node's own brand). It returns the domain as
// updated; failures are recorded on it (SyncError) and logged, and the last
// good brand stays in place.
func (s *Server) SyncBrandingDomain(ctx context.Context, d *statedb.BrandingDomain) *statedb.BrandingDomain {
	defer s.lockDomain(d.Hostname)()
	return s.syncDomainLocked(ctx, d)
}

func (s *Server) syncDomainLocked(ctx context.Context, d *statedb.BrandingDomain) *statedb.BrandingDomain {
	ctx = log.WithLogValues(ctx, "hostname", d.Hostname, "owner", d.OwnerDID)
	var cidStr string
	values := branding.Values{}
	var warnings []string
	fetched, err := branding.FetchRecord(ctx, atproto.CustomDirectory(s.cli.PLCURL), &aqhttp.Client, d.OwnerDID, d.Hostname)
	switch {
	case errors.Is(err, branding.ErrNoRecord):
		err = nil
	case err == nil:
		cidStr, values, warnings = fetched.CID, fetched.Values, fetched.Warnings
	}
	// The fetch takes a while; the domain may have been removed or handed to
	// someone else meanwhile (by another node of the station, which this
	// node's lock does not cover). Never cache a brand for a domain its
	// author no longer owns.
	cur, cerr := s.statefulDB.GetBrandingDomain(d.Hostname)
	if cerr == nil && (cur == nil || cur.OwnerDID != d.OwnerDID) {
		log.Log(ctx, "custom domain changed hands during sync; dropping the fetched brand")
		if cur == nil {
			return d
		}
		return cur
	}
	if err == nil {
		var report *branding.Report
		report, err = branding.Apply(ctx, s.statefulDB, d.BrandID(), values, warnings, false, false)
		if err == nil {
			changed := 0
			for _, c := range report.Changes {
				if c.Action != "unchanged" {
					changed++
				}
			}
			log.Log(ctx, "custom domain brand synced", "cid", cidStr, "changed", changed, "warnings", len(report.Warnings))
		}
	}
	if err != nil {
		log.Warn(ctx, "custom domain brand sync failed", "err", err)
	}
	if merr := s.statefulDB.MarkBrandingDomainSynced(d.Hostname, cidStr, err); merr != nil {
		log.Error(ctx, "failed to record custom domain sync", "err", merr)
	}
	if fresh, gerr := s.statefulDB.GetBrandingDomain(d.Hostname); gerr == nil && fresh != nil {
		return fresh
	}
	return d
}

// SyncBrandingDomainsFor re-pulls the custom domain ownerDID owns whose
// hostname is rkey, if there is one. The firehose calls it inline for every
// brand record on the network, so the check is a cached lookup and only a
// match goes on to the (slow) pull, in the background.
func (s *Server) SyncBrandingDomainsFor(ctx context.Context, ownerDID, rkey string) {
	d, err := s.statefulDB.GetBrandingDomain(rkey)
	if err != nil || d == nil || d.OwnerDID != ownerDID {
		return
	}
	go s.SyncBrandingDomain(context.WithoutCancel(ctx), d)
}

// SyncBrandingDomains re-pulls every custom domain's brand every interval
// until ctx ends: the backstop for firehose events a node missed (or never
// sees, with the firehose off).
func (s *Server) SyncBrandingDomains(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		domains, err := s.statefulDB.ListBrandingDomains("")
		if err != nil {
			log.Warn(ctx, "listing custom domains failed", "err", err)
		}
		for i := range domains {
			s.SyncBrandingDomain(ctx, &domains[i])
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// brandTarget is a branding write resolved: which brand, and for a custom
// domain the owner's session to publish the record with.
type brandTarget struct {
	brandID string
	domain  *statedb.BrandingDomain
	client  *oatproxy.XrpcClient
	caller  string
}

// brandWriteTarget authorizes a branding write addressed to the broadcaster
// param. The node's own brand (or any non-domain ID) takes an admin; a
// custom domain's brand takes its owner, since the change is a write to the
// owner's repo. Admins manage who owns a domain, not what it looks like.
func (s *Server) brandWriteTarget(ctx context.Context, param string) (*brandTarget, error) {
	session, client := oatproxy.GetOAuthSession(ctx)
	if session == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session not found")
	}
	t := &brandTarget{brandID: s.getBroadcasterID(ctx, param), caller: session.DID}
	t.domain = branding.DomainForBrandID(s.statefulDB, t.brandID)
	if t.domain == nil {
		if !s.isAdminDID(session.DID) {
			log.Warn(ctx, "unauthorized branding write", "did", session.DID, "brand", t.brandID)
			return nil, echo.NewHTTPError(http.StatusUnauthorized, "not authorized to modify branding")
		}
		return t, nil
	}
	if session.DID != t.domain.OwnerDID {
		log.Warn(ctx, "unauthorized custom domain branding write", "did", session.DID, "hostname", t.domain.Hostname)
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "not authorized to modify branding: only the domain's owner can change its brand")
	}
	if client == nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "oauth session has no client")
	}
	t.client = client
	return t, nil
}

// editDomainBrand changes a custom domain's brand: edit gets the brand as
// the owner's record has it now and returns what to publish. The domain is
// locked from the pull to the publish, so two edits through this node
// apply one after the other; the publish is a compare-and-swap on the
// record version the edit started from, so an edit made elsewhere in the
// meantime (another node, another client) fails the write instead of being
// silently overwritten.
func (s *Server) editDomainBrand(ctx context.Context, t *brandTarget, edit func(branding.Values) (branding.Values, error)) error {
	defer s.lockDomain(t.domain.Hostname)()
	d := s.syncDomainLocked(ctx, t.domain)
	if d.SyncError != "" {
		return echo.NewHTTPError(http.StatusBadGateway, "unable to read the brand record: "+d.SyncError)
	}
	if d.OwnerDID != t.domain.OwnerDID {
		return echo.NewHTTPError(http.StatusConflict, "the domain changed hands")
	}
	current, err := branding.ReadValues(s.statefulDB, t.brandID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "unable to read branding")
	}
	next, err := edit(current)
	if err != nil || next == nil {
		return err
	}
	if err := s.publishDomainBrand(ctx, t, next, d.RecordCID); err != nil {
		log.Error(ctx, "failed to publish custom domain brand", "err", err)
		// The PDS refuses a stale swapRecord with InvalidSwap, and a
		// createRecord on a key someone just created as already existing.
		if msg := err.Error(); strings.Contains(msg, "InvalidSwap") || strings.Contains(strings.ToLower(msg), "already exists") {
			return echo.NewHTTPError(http.StatusConflict, "the brand record changed while this edit was being made; reload and try again")
		}
		return echo.NewHTTPError(http.StatusBadGateway, "unable to publish brand record: "+err.Error())
	}
	return nil
}

// publishDomainBrand writes values as the domain's brand record in the
// owner's repo, then caches them. The record goes first: if the owner's PDS
// refuses it, nothing changes here either. base is the record version the
// values were derived from ("" when there was none).
func (s *Server) publishDomainBrand(ctx context.Context, t *brandTarget, values branding.Values, base string) error {
	upload := func(ctx context.Context, key string, v branding.Value) (*glex.Blob, error) {
		var out comatproto.RepoUploadBlob_Output
		if err := t.client.Do(ctx, xrpc.Procedure, v.MimeType, "com.atproto.repo.uploadBlob", nil, bytes.NewReader(v.Data), &out); err != nil {
			return nil, fmt.Errorf("upload: %w", err)
		}
		return &out.Blob, nil
	}
	rec, err := branding.ToRecord(ctx, values, upload)
	if err != nil {
		return err
	}
	input := map[string]any{
		"repo":       t.domain.OwnerDID,
		"collection": branding.RecordNSID,
		"rkey":       t.domain.Hostname,
		"record":     rec,
	}
	// Compare and swap: putRecord against the version we started from, or
	// createRecord (which refuses an existing key) when there was none.
	method := "com.atproto.repo.createRecord"
	if base != "" {
		method = "com.atproto.repo.putRecord"
		input["swapRecord"] = base
	}
	var out comatproto.RepoPutRecord_Output
	if err := t.client.Do(ctx, xrpc.Procedure, "application/json", method, nil, input, &out); err != nil {
		return fmt.Errorf("publish brand record: %w", err)
	}
	if _, err := branding.Apply(ctx, s.statefulDB, t.brandID, values, nil, false, false); err != nil {
		return err
	}
	if err := s.statefulDB.MarkBrandingDomainSynced(t.domain.Hostname, out.Cid, nil); err != nil {
		log.Warn(ctx, "failed to record custom domain sync", "err", err)
	}
	log.Log(ctx, "custom domain brand published", "hostname", t.domain.Hostname, "uri", out.Uri, "cid", out.Cid)
	return nil
}
