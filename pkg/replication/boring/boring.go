package boring

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"stream.place/streamplace/pkg/aqhttp"
	"stream.place/streamplace/pkg/log"
)

// boring HTTP replication mechanism
type BoringReplicator struct {
	Peers []string
}

func (rep *BoringReplicator) NewSegment(ctx context.Context, bs []byte) {
	for _, p := range rep.Peers {
		go func(peer string) {
			ctx := log.WithLogValues(ctx, "peer", peer)
			err := sendSegment(ctx, peer, bs)
			if err != nil {
				log.Log(ctx, "error replicating segment", "error", err)
			}
		}(p)
	}
}

func sendSegment(ctx context.Context, peer string, bs []byte) error {
	r := bytes.NewReader(bs)
	peerURL := fmt.Sprintf("%s/api/segment", peer)
	req, err := http.NewRequestWithContext(ctx, "POST", peerURL, r)
	if err != nil {
		return err
	}
	res, err := aqhttp.Client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode != 204 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("unexpected http code %d body=%s", res.StatusCode, body)
	}
	return nil
}
