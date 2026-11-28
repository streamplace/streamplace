package atprotoauth

import (
	"net/url"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
)

// RepoGrant reports whether a repo scope token grants the collection and action.
// An empty action requires an unrestricted grant.
func RepoGrant(token, collection, action string, wildcard bool) bool {
	switch action {
	case "", "create", "update", "delete":
	default:
		return false
	}
	base, query, hasQuery := strings.Cut(token, "?")
	positional, hasPositional := strings.CutPrefix(base, "repo:")
	if !hasPositional && base != "repo" {
		return false
	}
	if _, err := syntax.ParseNSID(collection); err != nil {
		return false
	}
	matchesCollection := false
	if hasPositional {
		if positional != collection && (!wildcard || positional != "*") {
			return false
		}
		matchesCollection = true
	}
	if !hasQuery {
		return matchesCollection
	}

	var hasAction, matchesAction bool
	for param := range strings.SplitSeq(query, "&") {
		key, value, ok := strings.Cut(param, "=")
		if !ok {
			return false
		}
		key, err := url.QueryUnescape(key)
		if err != nil {
			return false
		}
		value, err = url.QueryUnescape(value)
		if err != nil {
			return false
		}
		switch key {
		case "collection":
			if hasPositional {
				return false
			}
			if value != "*" {
				if _, err := syntax.ParseNSID(value); err != nil {
					return false
				}
				matchesCollection = matchesCollection || value == collection
			} else if wildcard {
				matchesCollection = true
			}
		case "action":
			switch value {
			case "create", "update", "delete":
				hasAction = true
				matchesAction = matchesAction || value == action
			default:
				return false
			}
		default:
			return false
		}
	}
	return matchesCollection && (!hasAction || matchesAction)
}
