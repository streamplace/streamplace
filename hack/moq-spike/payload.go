package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	muxl "github.com/streamplace/muxl/go"
)

// segment is one authentic S2PA-signed MUXL canonical segment: the per-GoP
// bare .m4s Streamplace stores and ships, every track's
// [c2pa-uuid][muxl-uuid][moof][mdat] concatenated in track-id order (the same
// assembly pkg/muxl's verify/wrap tests perform).
type segment struct {
	payload    []byte
	sha256     string
	durationUs uint64
}

// signer is an ephemeral secp256k1 identity whose leaf cert muxl mints: the
// shape pkg/media.NewEphemeralMediaSigner builds for the shadow ingest, minus
// the atproto DID plumbing. The key never leaves the host; muxl calls back
// into sign for every segment, the "host-callback" backend production uses
// for non-software keys.
type signer struct {
	sign    func([]byte) ([]byte, error)
	certPEM []byte
}

func newEphemeralSigner(ctx context.Context, eng muxl.Engine) (*signer, error) {
	priv, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	sign := muxl.SignerToCallback(priv.ToECDSA(), 32)
	certPEM, err := eng.GenCert(ctx, muxl.CertInput{
		PubKey: priv.PubKey().SerializeUncompressed(),
		Sign:   sign,
	})
	if err != nil {
		return nil, fmt.Errorf("mint cert: %w", err)
	}
	return &signer{sign: sign, certPEM: certPEM}, nil
}

// spikeManifest is the C2PA manifest every segment is signed against; the
// minimal c2pa.created shape pkg/muxl's tests sign with.
var spikeManifest = []byte(`{"title":"moq-spike","assertions":[{"label":"c2pa.actions","data":{"actions":[{"action":"c2pa.created"}]}}]}`)

// signFixture canonicalizes an MP4 fixture into MUXL and S2PA-signs it per
// GoP, returning one segment per GoP. The bytes are exactly what muxl-sign
// emitted; the only check here refuses anything that is not a signed
// canonical segment (a uuid box leads every track), so a fixture the toolchain
// could not sign fails loudly instead of shipping unsigned bytes.
func signFixture(ctx context.Context, eng muxl.Engine, s *signer, path string) ([]segment, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	canon, err := eng.Canonicalize(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("canonicalize %s: %w", path, err)
	}

	events := make(chan *muxl.Event, 16)
	errCh := make(chan error, 1)
	go func() {
		errCh <- eng.SignSegment(ctx, bytes.NewReader(canon), muxl.SignerInput{
			CertPEM:         s.certPEM,
			Sign:            s.sign,
			TrackManifest:   spikeManifest,
			WrapperManifest: spikeManifest,
		}, nil, nil, events)
		close(events)
	}()

	var segs []segment
	for ev := range events {
		if ev.Type != "signed-segment" {
			continue
		}
		tids := make([]string, 0, len(ev.Tracks))
		for tid := range ev.Tracks {
			tids = append(tids, tid)
		}
		sort.Strings(tids)
		var payload []byte
		for _, tid := range tids {
			track := ev.Tracks[tid]
			if len(track) < 8 || string(track[4:8]) != "uuid" {
				return nil, fmt.Errorf("%s: GoP %d track %s is not a signed canonical segment", path, ev.Number, tid)
			}
			payload = append(payload, track...)
		}
		sum := sha256.Sum256(payload)
		segs = append(segs, segment{payload: payload, sha256: hex.EncodeToString(sum[:]), durationUs: ev.DurationUs})
	}
	if err := <-errCh; err != nil {
		return nil, fmt.Errorf("sign %s: %w", path, err)
	}
	if len(segs) == 0 {
		return nil, fmt.Errorf("%s: muxl produced no signed segments", path)
	}
	return segs, nil
}
