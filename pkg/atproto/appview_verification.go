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
// verifyAppViewField, verifyAppViewValues), and, with verifyBluesky on,
// Bluesky's public app view, whose getProfile carries the blue check
// (verification.verifiedStatus). There is no feed of changes to subscribe
// to, so the node asks:
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
	issuer string   // "" = did:web:Host
	Fields []string // dotted paths into the profile; any one matching counts
	Values []string // empty = any non-empty value
}

// Issuer is the verifier DID the app view's rows are filed under.
func (c appViewConfig) Issuer() string {
	if c.issuer != "" {
		return c.issuer
	}
	return "did:web:" + c.Host
}

// Bluesky's blue check: the public app view says verifiedStatus (a
// verified account) or trustedVerifierStatus (a verifier, whose check is
// scalloped) is valid.
const (
	blueskyAppViewURL    = "https://public.api.bsky.app"
	BlueskyAppViewIssuer = "did:web:api.bsky.app"
)

func blueskyAppViewConfig() appViewConfig {
	return appViewConfig{
		URL:    blueskyAppViewURL,
		Host:   "public.api.bsky.app",
		issuer: BlueskyAppViewIssuer,
		Fields: []string{"verification.verifiedStatus", "verification.trustedVerifierStatus"},
		Values: []string{"valid"},
	}
}

func parseAppViewConfig(rawURL, field, values string) appViewConfig {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return appViewConfig{}
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return appViewConfig{}
	}
	c := appViewConfig{URL: strings.TrimRight(rawURL, "/"), Host: u.Host}
	if field = strings.TrimSpace(field); field == "" {
		field = "wsocialVerified"
	}
	c.Fields = []string{field}
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

// verified reports whether a getProfile view counts as verified: any of
// the configured fields matches.
func (c appViewConfig) verified(profile map[string]any) bool {
	for _, f := range c.Fields {
		if c.matches(fieldValue(profile, f)) {
			return true
		}
	}
	return false
}

// fieldValue walks a dotted path ("verification.verifiedStatus") into a
// decoded JSON object; nil when any step is missing.
func fieldValue(obj map[string]any, path string) any {
	var cur any = obj
	for _, key := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[key]
	}
	return cur
}

// appViewVerificationURI is the synthetic record URI of a mirrored answer.
func appViewVerificationURI(host, did string) string {
	return fmt.Sprintf("appview://%s/%s", host, did)
}

var (
	appViewNegMu sync.Mutex
	appViewNeg   = map[string]time.Time{} // issuer+did -> when the "no" expires
	appViewGroup singleflight.Group
)

func appViewNegKey(c appViewConfig, did string) string { return c.Issuer() + " " + did }

// appViewLookup asks each app view about did, in order, if nothing vouches
// for it yet, stopping at the first yes. It returns true when one says
// verified (and the row has been written).
func (atsync *ATProtoSynchronizer) appViewLookup(ctx context.Context, did string) bool {
	if did == "" {
		return false
	}
	for _, c := range atsync.AppViews(ctx) {
		if atsync.appViewLookupOne(ctx, c, did) {
			return true
		}
	}
	return false
}

// appViewLookupOne asks one app view about did unless a recent "no" is
// remembered. Callers on hot paths share one in-flight request per DID.
func (atsync *ATProtoSynchronizer) appViewLookupOne(ctx context.Context, c appViewConfig, did string) bool {
	key := appViewNegKey(c, did)
	appViewNegMu.Lock()
	until, denied := appViewNeg[key]
	appViewNegMu.Unlock()
	if denied && time.Now().Before(until) {
		return false
	}
	v, _, _ := appViewGroup.Do(key, func() (any, error) {
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
	key := appViewNegKey(c, did)
	if verified {
		appViewNegMu.Lock()
		delete(appViewNeg, key)
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
	appViewNeg[key] = time.Now().Add(appViewNegativeTTL)
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
			out[did] = c.verified(p)
		}
	}
	return out, nil
}

// RefreshAppViewVerificationsForever re-asks each app view about every
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
		for _, c := range atsync.AppViews(ctx) {
			atsync.refreshAppView(ctx, c)
		}
	}
}

func (atsync *ATProtoSynchronizer) refreshAppView(ctx context.Context, c appViewConfig) {
	rows, err := atsync.Model.ListVerificationsByIssuer(ctx, c.Issuer())
	if err != nil {
		log.Warn(ctx, "failed to list app view verifications", "issuer", c.Issuer(), "err", err)
		return
	}
	dids := make([]string, 0, len(rows))
	for _, r := range rows {
		dids = append(dids, r.SubjectDID)
	}
	if len(dids) == 0 {
		return
	}
	res, err := atsync.checkAppView(ctx, c, dids)
	if err != nil {
		log.Warn(ctx, "app view verification refresh failed", "issuer", c.Issuer(), "err", err)
		return
	}
	revoked := 0
	for _, did := range dids {
		if !res[did] {
			atsync.applyAppViewAnswer(ctx, c, did, false)
			revoked++
		}
	}
	if revoked > 0 {
		log.Log(ctx, "app view verifications refreshed", "issuer", c.Issuer(), "checked", len(dids), "revoked", revoked)
	}
}
