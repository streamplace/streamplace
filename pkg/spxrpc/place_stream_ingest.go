package spxrpc

import (
	"context"
	"fmt"
	"net"
	"strings"

	placestream "stream.place/streamplace/pkg/placestream"
)

func hostPort(host, port, defaultPort string) string {
	if port == defaultPort {
		return host
	}
	return net.JoinHostPort(host, port)
}

func (s *Server) handlePlaceStreamIngestGetIngestUrls(ctx context.Context) (*placestream.IngestGetIngestUrls_Output, error) {
	if s.cli.Ingests != nil {
		return s.cli.Ingests, nil
	}
	broadcasterDID := s.cli.BroadcasterDID()
	broadcasterHost := strings.TrimPrefix(broadcasterDID, "did:web:")
	out := placestream.IngestGetIngestUrls_Output{
		Ingests: []placestream.IngestGetIngestUrls_Output_Ingests_Elem{},
	}
	if !s.cli.Secure {
		_, rtmpPort, err := net.SplitHostPort(s.cli.RTMPAddr)
		if err != nil {
			return nil, err
		}
		rtmpUrl := fmt.Sprintf("rtmp://%s/live", hostPort(broadcasterHost, rtmpPort, "1935"))
		out.Ingests = append(out.Ingests, placestream.IngestGetIngestUrls_Output_Ingests_Elem{
			IngestDefs_Ingest: &placestream.IngestDefs_Ingest{
				Type: "rtmp",
				Url:  rtmpUrl,
			},
		})

		_, httpPort, err := net.SplitHostPort(s.cli.HTTPAddr)
		if err != nil {
			return nil, err
		}
		whipUrl := fmt.Sprintf("http://%s", hostPort(broadcasterHost, httpPort, "80"))
		out.Ingests = append(out.Ingests, placestream.IngestGetIngestUrls_Output_Ingests_Elem{
			IngestDefs_Ingest: &placestream.IngestDefs_Ingest{
				Type: "whip",
				Url:  whipUrl,
			},
		})
	} else {
		_, rtmpsPort, err := net.SplitHostPort(s.cli.RTMPSAddr)
		if err != nil {
			return nil, err
		}
		rtmpsUrl := fmt.Sprintf("rtmps://%s:%s/live", broadcasterHost, rtmpsPort)
		out.Ingests = append(out.Ingests, placestream.IngestGetIngestUrls_Output_Ingests_Elem{
			IngestDefs_Ingest: &placestream.IngestDefs_Ingest{
				Type: "rtmps",
				Url:  rtmpsUrl,
			},
		})

		_, httpsPort, err := net.SplitHostPort(s.cli.HTTPSAddr)
		if err != nil {
			return nil, err
		}
		whipUrl := fmt.Sprintf("https://%s", hostPort(broadcasterHost, httpsPort, "443"))
		out.Ingests = append(out.Ingests, placestream.IngestGetIngestUrls_Output_Ingests_Elem{
			IngestDefs_Ingest: &placestream.IngestDefs_Ingest{
				Type: "whip",
				Url:  whipUrl,
			},
		})
	}

	return &out, nil
}
