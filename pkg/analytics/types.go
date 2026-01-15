package analytics

import (
	pb "stream.place/streamplace/pkg/analytics/pb"
)

// Event represents an analytics event to be ingested
type Event struct {
	EventID        string
	EventType      string
	DeviceID       string
	DID            *string
	SessionID      string
	TimestampMs    int64
	StreamerDID    string
	StreamID       *string
	PropertiesJSON string
	SchemaVersion  uint16
	ClientVersion  string
	Platform       string
}

// RealtimeStatsRequest requests real-time statistics
type RealtimeStatsRequest struct {
	StreamerDid   *string
	WindowMinutes uint32
}

// RealtimeStatsResponse contains real-time statistics
type RealtimeStatsResponse struct {
	Streamers []StreamerRealtimeStats `json:"streamers"`
}

// StreamerRealtimeStats contains real-time stats for a single streamer
type StreamerRealtimeStats struct {
	StreamerDid      string `json:"streamerDid"`
	CurrentViewers   uint64 `json:"currentViewers"`
	TotalWatchTimeMs uint64 `json:"totalWatchTimeMs"`
}

// StreamerStatsRequest requests stats for a specific streamer
type StreamerStatsRequest struct {
	StreamerDid string
	StartTimeMs *int64
	EndTimeMs   *int64
}

// StreamerStatsResponse contains aggregated stats for a streamer
type StreamerStatsResponse struct {
	StreamerDid      string       `json:"streamerDid"`
	TotalViews       uint64       `json:"totalViews"`
	TotalWatchTimeMs uint64       `json:"totalWatchTimeMs"`
	UniqueViewers    uint64       `json:"uniqueViewers"`
	DailyStats       []DailyStats `json:"dailyStats"`
}

// DailyStats contains stats for a single day
type DailyStats struct {
	Date          string `json:"date"`
	Views         uint64 `json:"views"`
	WatchTimeMs   uint64 `json:"watchTimeMs"`
	UniqueViewers uint64 `json:"uniqueViewers"`
}

// ViewerHistoryRequest requests viewing history for a user
type ViewerHistoryRequest struct {
	Did         string
	StartTimeMs *int64
	EndTimeMs   *int64
	Limit       uint32
}

// ViewerHistoryResponse contains viewing history
type ViewerHistoryResponse struct {
	Sessions []WatchSession `json:"sessions"`
}

// WatchSession represents a single viewing session
type WatchSession struct {
	SessionId   string  `json:"sessionId"`
	StreamerDid string  `json:"streamerDid"`
	StreamId    *string `json:"streamId,omitempty"`
	StartTimeMs int64   `json:"startTimeMs"`
	EndTimeMs   int64   `json:"endTimeMs"`
	DurationMs  uint64  `json:"durationMs"`
}

// Conversion helpers

func convertRealtimeStatsResponse(pbResp *pb.RealtimeStatsResponse) *RealtimeStatsResponse {
	if pbResp == nil {
		return &RealtimeStatsResponse{}
	}

	streamers := make([]StreamerRealtimeStats, len(pbResp.Streamers))
	for i, s := range pbResp.Streamers {
		streamers[i] = StreamerRealtimeStats{
			StreamerDid:      s.StreamerDid,
			CurrentViewers:   s.CurrentViewers,
			TotalWatchTimeMs: s.TotalWatchTimeMs,
		}
	}

	return &RealtimeStatsResponse{
		Streamers: streamers,
	}
}

func convertStreamerStatsResponse(pbResp *pb.StreamerStatsResponse) *StreamerStatsResponse {
	if pbResp == nil {
		return &StreamerStatsResponse{}
	}

	dailyStats := make([]DailyStats, len(pbResp.DailyStats))
	for i, s := range pbResp.DailyStats {
		dailyStats[i] = DailyStats{
			Date:          s.Date,
			Views:         s.Views,
			WatchTimeMs:   s.WatchTimeMs,
			UniqueViewers: s.UniqueViewers,
		}
	}

	return &StreamerStatsResponse{
		StreamerDid:      pbResp.StreamerDid,
		TotalViews:       pbResp.TotalViews,
		TotalWatchTimeMs: pbResp.TotalWatchTimeMs,
		UniqueViewers:    pbResp.UniqueViewers,
		DailyStats:       dailyStats,
	}
}

func convertViewerHistoryResponse(pbResp *pb.ViewerHistoryResponse) *ViewerHistoryResponse {
	if pbResp == nil {
		return &ViewerHistoryResponse{}
	}

	sessions := make([]WatchSession, len(pbResp.Sessions))
	for i, s := range pbResp.Sessions {
		sessions[i] = WatchSession{
			SessionId:   s.SessionId,
			StreamerDid: s.StreamerDid,
			StreamId:    s.StreamId,
			StartTimeMs: s.StartTimeMs,
			EndTimeMs:   s.EndTimeMs,
			DurationMs:  s.DurationMs,
		}
	}

	return &ViewerHistoryResponse{
		Sessions: sessions,
	}
}
