package atproto

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
)

// App-view verification: a network whose notion of "verified" is a field on
// its own app view's getProfile (branding keys verifyAppViewUrl,
// verifyAppViewField, verifyAppViewValues). There is no feed of changes to
// subscribe to, so the node asks:
//
//   - on first sight of an account (a chat message, a getStatus for it), a
//     synchronous lookup with a short timeout, so a verified viewer's very
//     first message is shown rather than dropped;
//   - a negative answer is remembered for appViewNegativeTTL, after which
//     the next sighting asks again — a newly verified account gets through
//     within a minute of its next message or composer poll;
//   - positives are mirrored into the verification table under the app
//     view's did:web and re-checked in batches on a schedule, so a
//     revocation is honoured within appViewRefreshInterval.

const (
	appViewLookupTimeout   = 3 * time.Second
	appViewNegativeTTL     = time.Minute
	appViewRefreshInterval = 5 * time.Minute
	appViewBatch           = 25
)

type appViewConfig struct {
	URL    string
	Host   string
	Field  string
	Values []string // empty = any non-empty value
}

// Issuer is the verifier DID the app view's rows are filed under.
func (c appViewConfig) Issuer() string { return "did:web:" + c.Host }

func parseAppViewConfig(rawURL, field, values string) appViewConfig {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return appViewConfig{}
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return appViewConfig{}
	}
	c := appViewConfig{URL: strings.TrimRight(rawURL, "/"), Host: u.Host, Field: strings.TrimSpace(field)}
	if c.Field == "" {
		c.Field = "wsocialVerified"
	}
	for _, v := range strings.Split(values, ",") {
		if v = strings.TrimSpace(v); v != "" && v != "*" {
			c.Values = append(c.Values, v)
		}
	}
	return c
}

// matches reports whether a field value counts as verified.
func (c appViewConfig) matches(v any) bool {
	s, _ := v.(string)
	if s == "" {
		return false
	}
	if len(c.Values) == 0 {
		return true
	}
	for _, want := range c.Values {
		if s == want {
			return true
		}
	}
	return false
}

// appViewVerificationURI is the synthetic record URI of a mirrored answer.
func appViewVerificationURI(host, did string) string {
	return fmt.Sprintf("appview://%s/%s", host, did)
}

var (
	appViewNegMu sync.Mutex
	appViewNeg   = map[string]time.Time{} // did -> when the "no" expires
	appViewGroup singleflight.Group
)

// appViewLookup asks the app view about did if nothing vouches for it yet
// and no recent "no" is remembered. It returns true when the app view says
// verified (and the row has been written). Callers on hot paths share one
// in-flight request per DID.
func (atsync *ATProtoSynchronizer) appViewLookup(ctx context.Context, did string) bool {
	c := atsync.AppView(ctx)
	if c.URL == "" || did == "" {
		return false
	}
	appViewNegMu.Lock()
	until, denied := appViewNeg[did]
	appViewNegMu.Unlock()
	if denied && time.Now().Before(until) {
		return false
	}
	v, _, _ := appViewGroup.Do(did, func() (any, error) {
		lctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), appViewLookupTimeout)
		defer cancel()
		res, err := atsync.checkAppView(lctx, c, []string{did})
		if err != nil {
			log.Warn(ctx, "app view verification lookup failed", "did", did, "err", err)
			return false, nil
		}
		ok := res[did]
		atsync.applyAppViewAnswer(lctx, c, did, ok)
		return ok, nil
	})
	ok, _ := v.(bool)
	return ok
}

// applyAppViewAnswer records one answer: a row for yes, a remembered no
// (and no row) for no.
func (atsync *ATProtoSynchronizer) applyAppViewAnswer(ctx context.Context, c appViewConfig, did string, verified bool) {
	uri := appViewVerificationURI(c.Host, did)
	if verified {
		appViewNegMu.Lock()
		delete(appViewNeg, did)
		appViewNegMu.Unlock()
		err := atsync.Model.CreateVerification(ctx, &model.Verification{
			URI:        uri,
			IssuerDID:  c.Issuer(),
			SubjectDID: did,
			CreatedAt:  time.Now(),
		})
		if err != nil && err != model.ErrAlreadyIndexed {
			log.Warn(ctx, "failed to record app view verification", "did", did, "err", err)
		}
		return
	}
	appViewNegMu.Lock()
	appViewNeg[did] = time.Now().Add(appViewNegativeTTL)
	appViewNegMu.Unlock()
	if err := atsync.Model.DeleteVerification(ctx, uri); err != nil {
		log.Warn(ctx, "failed to clear app view verification", "did", did, "err", err)
	}
}

type appViewProfiles struct {
	Profiles []map[string]any `json:"profiles"`
}

// checkAppView asks the app view about up to appViewBatch DIDs at once and
// reports which are verified. Accounts the app view doesn't know come back
// false.
func (atsync *ATProtoSynchronizer) checkAppView(ctx context.Context, c appViewConfig, dids []string) (map[string]bool, error) {
	out := map[string]bool{}
	for start := 0; start < len(dids); start += appViewBatch {
		end := start + appViewBatch
		if end > len(dids) {
			end = len(dids)
		}
		q := url.Values{}
		for _, d := range dids[start:end] {
			q.Add("actors", d)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/xrpc/app.bsky.actor.getProfiles?"+q.Encode(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := SyncHTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		var body appViewProfiles
		err = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("app view getProfiles: HTTP %d", resp.StatusCode)
		}
		if err != nil {
			return nil, fmt.Errorf("app view getProfiles: %w", err)
		}
		for _, p := range body.Profiles {
			did, _ := p["did"].(string)
			if did == "" {
				continue
			}
			out[did] = c.matches(p[c.Field])
		}
	}
	return out, nil
}

// RefreshAppViewVerificationsForever re-asks the app view about every
// account it has vouched for, in batches, so a verification the network
// withdraws stops counting here within appViewRefreshInterval. Accounts
// that were never verified are re-asked on sight instead (see
// appViewLookup), which is what makes a newly verified viewer's chat work
// without anyone restarting anything.
func (atsync *ATProtoSynchronizer) RefreshAppViewVerificationsForever(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(appViewRefreshInterval):
		}
		c := atsync.AppView(ctx)
		if c.URL == "" {
			continue
		}
		rows, err := atsync.Model.ListVerificationsByIssuer(ctx, c.Issuer())
		if err != nil {
			log.Warn(ctx, "failed to list app view verifications", "err", err)
			continue
		}
		dids := make([]string, 0, len(rows))
		for _, r := range rows {
			dids = append(dids, r.SubjectDID)
		}
		if len(dids) == 0 {
			continue
		}
		res, err := atsync.checkAppView(ctx, c, dids)
		if err != nil {
			log.Warn(ctx, "app view verification refresh failed", "err", err)
			continue
		}
		revoked := 0
		for _, did := range dids {
			if !res[did] {
				atsync.applyAppViewAnswer(ctx, c, did, false)
				revoked++
			}
		}
		if revoked > 0 {
			log.Log(ctx, "app view verifications refreshed", "checked", len(dids), "revoked", revoked)
		}
	}
}
