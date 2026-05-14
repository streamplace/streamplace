package spxrpc

import (
	"stream.place/streamplace/pkg/atproto"
)

// accountBanned reports whether the account identified by did carries an
// active ban-worthy label (the same set that gates live stream keys via
// atproto.IsBanned). A banned account can't upload new videos and all of
// its existing VODs stop being served.
func (s *Server) accountBanned(did string) (bool, error) {
	labels, err := s.model.GetActiveLabels(did)
	if err != nil {
		return false, err
	}
	return atproto.IsBanned(labels...), nil
}

// recordLabeled reports whether the record at uri carries any active
// label at all. Any label suppresses serving — matching how a labeled
// chat message is hidden from the feed regardless of the label value.
func (s *Server) recordLabeled(uri string) (bool, error) {
	labels, err := s.model.GetActiveLabels(uri)
	if err != nil {
		return false, err
	}
	return len(labels) > 0, nil
}
