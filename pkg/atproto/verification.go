package atproto

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/bluesky-social/indigo/xrpc"
	"stream.place/streamplace/pkg/appbsky"
	"stream.place/streamplace/pkg/aqtime"
	"stream.place/streamplace/pkg/constants"
	"stream.place/streamplace/pkg/log"
	"stream.place/streamplace/pkg/model"
	"stream.place/streamplace/pkg/placestream"
)

// Chat access on this node is decided per streamer by their
// place.stream.chat.access records: allow and deny rules whose subjects name
// a verifier (accounts holding one of its app.bsky.graph.verification
// records) or a labeler and label. Nothing is configured on the node: the
// verifiers and labelers to index are whatever the indexed rules name, and a
// rule naming a new one starts its indexing right away. A verification from
// a verifier the streamer allows is what puts the badge on a chat author,
// on that streamer's stream.

const rulesCacheTTL = 15 * time.Second

var (
	verifierMu       sync.Mutex
	rulesCache       = map[string]cachedRules{}
	subjectsAt       time.Time
	verifiersCached  []string
	labelersCached   map[string][]string
	seedVerifierNow  = make(chan string, 64)
	seededVerifierMu sync.Mutex
	seededVerifiers  = map[string]bool{}
)

type cachedRules struct {
	rules []model.ChatAccessRule
	at    time.Time
}

// rulesFor returns the streamer's rules, cached briefly. An error is a
// lookup failure, which callers must not read as "no rules".
func (atsync *ATProtoSynchronizer) rulesFor(ctx context.Context, streamer string) ([]model.ChatAccessRule, error) {
	if streamer == "" || atsync.Model == nil {
		return nil, nil
	}
	verifierMu.Lock()
	if c, ok := rulesCache[streamer]; ok && time.Since(c.at) < rulesCacheTTL {
		verifierMu.Unlock()
		return c.rules, nil
	}
	verifierMu.Unlock()
	rules, err := atsync.Model.ListChatAccessRules(ctx, streamer)
	if err != nil {
		log.Error(ctx, "failed to list chat access rules", "streamer", streamer, "err", err)
		return nil, err
	}
	verifierMu.Lock()
	rulesCache[streamer] = cachedRules{rules: rules, at: time.Now()}
	verifierMu.Unlock()
	return rules, nil
}

func (atsync *ATProtoSynchronizer) refreshSubjects(ctx context.Context) {
	verifierMu.Lock()
	fresh := time.Since(subjectsAt) < 2*rulesCacheTTL
	verifierMu.Unlock()
	if fresh || atsync.Model == nil {
		return
	}
	verifiers, labelers, err := atsync.Model.ChatAccessSubjects(ctx)
	if err != nil {
		log.Error(ctx, "failed to list chat access subjects", "err", err)
		return
	}
	verifierMu.Lock()
	verifiersCached, labelersCached, subjectsAt = verifiers, labelers, time.Now()
	verifierMu.Unlock()
}

// Verifiers returns every verifier DID some streamer's rule names: the
// repos whose app.bsky.graph.verification records this node indexes.
func (atsync *ATProtoSynchronizer) Verifiers(ctx context.Context) []string {
	atsync.refreshSubjects(ctx)
	verifierMu.Lock()
	defer verifierMu.Unlock()
	return verifiersCached
}

// Labelers returns, per labeler DID some streamer's rule names, the label
// values (exact, or prefix with a trailing *) that this node mirrors.
func (atsync *ATProtoSynchronizer) Labelers(ctx context.Context) map[string][]string {
	atsync.refreshSubjects(ctx)
	verifierMu.Lock()
	defer verifierMu.Unlock()
	return labelersCached
}

// NoteChatAccessRule is called when a rule is indexed or deleted: the caches
// drop, and a verifier this node has not seeded yet is seeded now rather
// than at the next hourly pass.
func (atsync *ATProtoSynchronizer) NoteChatAccessRule(ctx context.Context, rule *model.ChatAccessRule) {
	verifierMu.Lock()
	delete(rulesCache, rule.RepoDID)
	subjectsAt = time.Time{}
	verifierMu.Unlock()
	if rule.SubjectType == model.ChatAccessSubjectVerifier {
		seededVerifierMu.Lock()
		seen := seededVerifiers[rule.SubjectDID]
		seededVerifierMu.Unlock()
		if !seen {
			select {
			case seedVerifierNow <- rule.SubjectDID:
			default:
			}
		}
	}
}

// labelValueOf returns the label value a mirrored label row stands for
// (its URI is label://<labeler>/<subject>/<value>), or "" for a real
// verification record.
func labelValueOf(v model.Verification) string {
	if !strings.HasPrefix(v.URI, "label://") {
		return ""
	}
	return v.URI[strings.LastIndex(v.URI, "/")+1:]
}

// ruleMatches reports whether one of the subject's verifications satisfies
// the rule's subject.
func ruleMatches(rule model.ChatAccessRule, verifications []model.Verification) bool {
	for _, v := range verifications {
		if v.IssuerDID != rule.SubjectDID {
			continue
		}
		switch rule.SubjectType {
		case model.ChatAccessSubjectVerifier:
			if labelValueOf(v) == "" {
				return true
			}
		case model.ChatAccessSubjectLabel:
			if val := labelValueOf(v); val != "" && labelMatches(val, []string{rule.LabelValue}) {
				return true
			}
		}
	}
	return false
}

func issuersOf(rules []model.ChatAccessRule) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rules {
		if !seen[r.SubjectDID] {
			seen[r.SubjectDID] = true
			out = append(out, r.SubjectDID)
		}
	}
	return out
}

// verificationsUnder returns did's verifications by the issuers the rules
// name, or nil.
func (atsync *ATProtoSynchronizer) verificationsUnder(ctx context.Context, rules []model.ChatAccessRule, did string) []model.Verification {
	issuers := issuersOf(rules)
	if len(issuers) == 0 || did == "" {
		return nil
	}
	found, err := atsync.Model.VerificationsFor(ctx, []string{did}, issuers)
	if err != nil {
		log.Error(ctx, "failed to look up verifications", "did", did, "err", err)
		return nil
	}
	return atsync.currentVerifications(ctx, did, found[did])
}

// currentVerifications drops the app.bsky.graph.verification records that
// no longer describe the account: the record vouches for a handle and a
// display name, and either changing invalidates it (that is the record's
// contract, and what the app view does). Mirrored labels carry neither and
// pass through.
func (atsync *ATProtoSynchronizer) currentVerifications(ctx context.Context, did string, vs []model.Verification) []model.Verification {
	var out []model.Verification
	var handle, displayName string
	looked := false
	for _, v := range vs {
		if labelValueOf(v) != "" || (v.Handle == "" && v.DisplayName == "") {
			out = append(out, v)
			continue
		}
		if !looked {
			looked = true
			handle = atsync.ResolveAuthorHandle(ctx, did)
			if atsync.Model != nil {
				if p, err := atsync.Model.GetBskyProfile(ctx, did, false); err == nil && p != nil && p.DisplayName != nil {
					displayName = *p.DisplayName
				}
			}
		}
		if v.Handle != "" && handle != "" && !strings.EqualFold(v.Handle, handle) {
			continue
		}
		if v.DisplayName != "" && displayName != "" && v.DisplayName != displayName {
			continue
		}
		out = append(out, v)
	}
	return out
}

// ChatAllowed reports whether did may chat on streamer's stream under the
// streamer's rules: with no rules everyone may; a matching deny rule always
// refuses; when any allow rule exists, only a match on one of them admits.
func (atsync *ATProtoSynchronizer) ChatAllowed(ctx context.Context, streamer, did string) bool {
	rules, err := atsync.rulesFor(ctx, streamer)
	if err != nil {
		// Fail closed: a database hiccup must not open a locked chat.
		return false
	}
	if len(rules) == 0 {
		return true
	}
	vs := atsync.verificationsUnder(ctx, rules, did)
	hasAllow := false
	allowed := false
	for _, r := range rules {
		match := ruleMatches(r, vs)
		switch r.Action {
		case model.ChatAccessDeny:
			if match {
				return false
			}
		case model.ChatAccessAllow:
			hasAllow = true
			if match {
				allowed = true
			}
		}
	}
	return !hasAllow || allowed
}

// VerificationState is did's verification, on streamer's stream, by the
// verifiers and labelers the streamer's allow rules name, in the shape the
// app view uses (so the app renders the same badge either way); nil when
// there is none.
func (atsync *ATProtoSynchronizer) VerificationState(ctx context.Context, streamer, did string) *appbsky.ActorDefs_VerificationState {
	var allows []model.ChatAccessRule
	rules, err := atsync.rulesFor(ctx, streamer)
	if err != nil {
		return nil
	}
	for _, r := range rules {
		if r.Action == model.ChatAccessAllow {
			allows = append(allows, r)
		}
	}
	if len(allows) == 0 {
		return nil
	}
	vs := atsync.verificationsUnder(ctx, allows, did)
	var state *appbsky.ActorDefs_VerificationState
	seen := map[string]bool{}
	for _, r := range allows {
		for _, v := range vs {
			if seen[v.URI] || !ruleMatches(r, []model.Verification{v}) {
				continue
			}
			seen[v.URI] = true
			if state == nil {
				state = &appbsky.ActorDefs_VerificationState{
					VerifiedStatus:        "valid",
					TrustedVerifierStatus: "none",
				}
			}
			state.Verifications = append(state.Verifications, appbsky.ActorDefs_VerificationView{
				CreatedAt: aqtime.FromTime(v.CreatedAt).String(),
				IsValid:   true,
				Issuer:    v.IssuerDID,
				Uri:       v.URI,
			})
		}
	}
	return state
}

// DecorateVerification fills the author's verification state on a chat
// message view for streamer's stream.
func (atsync *ATProtoSynchronizer) DecorateVerification(ctx context.Context, streamer string, message *placestream.ChatDefs_MessageView) {
	if message == nil {
		return
	}
	if state := atsync.VerificationState(ctx, streamer, message.Author.Did); state != nil {
		message.Author.Verification = state
	}
}

// SeedVerificationsForever pulls every verification record from each named
// verifier's repo: at boot, hourly after, and as soon as a rule names a
// verifier this node has not seeded. The firehose keeps the table current
// in between; the seeding covers records issued before this node existed
// (or while it was down).
func (atsync *ATProtoSynchronizer) SeedVerificationsForever(ctx context.Context) {
	seed := func(did string) {
		if err := atsync.seedVerifier(ctx, did); err != nil {
			log.Warn(ctx, "failed to seed verifications", "verifier", did, "err", err)
			return
		}
		seededVerifierMu.Lock()
		seededVerifiers[did] = true
		seededVerifierMu.Unlock()
	}
	for {
		for _, did := range atsync.Verifiers(ctx) {
			seed(did)
		}
		select {
		case <-ctx.Done():
			return
		case did := <-seedVerifierNow:
			seed(did)
		case <-time.After(time.Hour):
		}
	}
}

type listRecordsOut struct {
	Cursor  string `json:"cursor"`
	Records []struct {
		Uri   string          `json:"uri"`
		Cid   string          `json:"cid"`
		Value json.RawMessage `json:"value"`
	} `json:"records"`
}

func (atsync *ATProtoSynchronizer) seedVerifier(ctx context.Context, did string) error {
	ident, err := atsync.resolveIdent(ctx, did, true)
	if err != nil {
		return err
	}
	xrpcc := xrpc.Client{Host: ident.PDSEndpoint(), Client: SyncHTTPClient}
	if xrpcc.Host == "" {
		return nil
	}
	cursor := ""
	total := 0
	for {
		params := map[string]any{
			"repo":       did,
			"collection": constants.APP_BSKY_GRAPH_VERIFICATION,
			"limit":      100,
		}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var out listRecordsOut
		if err := xrpcc.Do(ctx, xrpc.Query, "", "com.atproto.repo.listRecords", params, nil, &out); err != nil {
			return err
		}
		for _, r := range out.Records {
			var rec appbsky.GraphVerification
			if err := json.Unmarshal(r.Value, &rec); err != nil || rec.Subject == "" {
				continue
			}
			v := &model.Verification{
				URI:         r.Uri,
				CID:         r.Cid,
				IssuerDID:   did,
				SubjectDID:  rec.Subject,
				Handle:      rec.Handle,
				DisplayName: rec.DisplayName,
			}
			if created, err := aqtime.FromString(rec.CreatedAt); err == nil {
				v.CreatedAt = created.Time()
			}
			if err := atsync.Model.CreateVerification(ctx, v); err != nil && err != model.ErrAlreadyIndexed {
				log.Warn(ctx, "failed to index seeded verification", "uri", r.Uri, "err", err)
				continue
			}
			total++
		}
		if out.Cursor == "" || len(out.Records) == 0 {
			break
		}
		cursor = out.Cursor
	}
	log.Log(ctx, "seeded verifications", "verifier", did, "records", total)
	return nil
}
