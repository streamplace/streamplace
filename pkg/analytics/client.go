package analytics

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "stream.place/streamplace/pkg/analytics/pb"
)

type Client interface {
	IngestEvents(ctx context.Context, events []*Event) error
	GetRealtimeStats(ctx context.Context, req *RealtimeStatsRequest) (*RealtimeStatsResponse, error)
	GetStreamerStats(ctx context.Context, req *StreamerStatsRequest) (*StreamerStatsResponse, error)
	GetViewerHistory(ctx context.Context, req *ViewerHistoryRequest) (*ViewerHistoryResponse, error)
	Close() error
}

type client struct {
	conn       *grpc.ClientConn
	grpcClient pb.AnalyticsClient
}

func NewClient(ctx context.Context, endpoint string) (Client, error) {

	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to analytics service at %s: %w", endpoint, err)
	}

	return &client{
		conn:       conn,
		grpcClient: pb.NewAnalyticsClient(conn),
	}, nil
}

func (c *client) IngestEvents(ctx context.Context, events []*Event) error {
	if len(events) == 0 {
		return nil
	}

	pbEvents := make([]*pb.Event, len(events))
	for i, e := range events {
		pbEvents[i] = &pb.Event{
			EventId:        e.EventID,
			EventType:      e.EventType,
			DeviceId:       e.DeviceID,
			Did:            e.DID,
			SessionId:      e.SessionID,
			TimestampMs:    e.TimestampMs,
			StreamerDid:    e.StreamerDID,
			StreamId:       e.StreamID,
			PropertiesJson: e.PropertiesJSON,
			SchemaVersion:  uint32(e.SchemaVersion),
			ClientVersion:  e.ClientVersion,
			Platform:       e.Platform,
		}
	}

	req := &pb.IngestEventsRequest{
		Events: pbEvents,
	}

	_, err := c.grpcClient.IngestEvents(ctx, req)
	return err
}

func (c *client) GetRealtimeStats(ctx context.Context, req *RealtimeStatsRequest) (*RealtimeStatsResponse, error) {
	pbReq := &pb.RealtimeStatsRequest{
		WindowMinutes: req.WindowMinutes,
		StreamerDid:   req.StreamerDid,
	}

	resp, err := c.grpcClient.GetRealtimeStats(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return convertRealtimeStatsResponse(resp), nil
}

func (c *client) GetStreamerStats(ctx context.Context, req *StreamerStatsRequest) (*StreamerStatsResponse, error) {
	pbReq := &pb.StreamerStatsRequest{
		StreamerDid: req.StreamerDid,
		StartTimeMs: req.StartTimeMs,
		EndTimeMs:   req.EndTimeMs,
	}

	resp, err := c.grpcClient.GetStreamerStats(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return convertStreamerStatsResponse(resp), nil
}

func (c *client) GetViewerHistory(ctx context.Context, req *ViewerHistoryRequest) (*ViewerHistoryResponse, error) {
	pbReq := &pb.ViewerHistoryRequest{
		Did:         req.Did,
		StartTimeMs: req.StartTimeMs,
		EndTimeMs:   req.EndTimeMs,
		Limit:       req.Limit,
	}

	resp, err := c.grpcClient.GetViewerHistory(ctx, pbReq)
	if err != nil {
		return nil, err
	}

	return convertViewerHistoryResponse(resp), nil
}

func (c *client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// NopClient is a no-op implementation of Client for when analytics is disabled
type nopClient struct{}

func (n *nopClient) IngestEvents(ctx context.Context, events []*Event) error {
	return nil
}

func (n *nopClient) GetRealtimeStats(ctx context.Context, req *RealtimeStatsRequest) (*RealtimeStatsResponse, error) {
	return &RealtimeStatsResponse{}, nil
}

func (n *nopClient) GetStreamerStats(ctx context.Context, req *StreamerStatsRequest) (*StreamerStatsResponse, error) {
	return &StreamerStatsResponse{}, nil
}

func (n *nopClient) GetViewerHistory(ctx context.Context, req *ViewerHistoryRequest) (*ViewerHistoryResponse, error) {
	return &ViewerHistoryResponse{}, nil
}

func (n *nopClient) Close() error {
	return nil
}
