package statedb

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// BrandingDomain is a custom domain: a hostname this node serves under the
// brand its owner publishes (their place.stream.branding.brand record keyed
// by the hostname) instead of the node's own. The brand's values are cached
// as BrandingBlob rows under did:web:<hostname>, refreshed from the record.
type BrandingDomain struct {
	Hostname  string `gorm:"primaryKey"`
	OwnerDID  string `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	// SyncedAt and SyncError describe the last pull of the owner's record;
	// RecordCID is the version the cached rows came from.
	SyncedAt  *time.Time
	SyncError string
	RecordCID string
}

// BrandID is the key the domain's branding rows are stored under.
func (d *BrandingDomain) BrandID() string {
	return "did:web:" + d.Hostname
}

// Every request resolves its Host against the domain list, so the list is
// held for brandingSnapshotTTL like the branding rows themselves.
type domainSnapshot struct {
	at      time.Time
	domains map[string]*BrandingDomain
}

func (state *StatefulDB) brandingDomains() (map[string]*BrandingDomain, error) {
	if v, ok := state.brandingCache.Load(domainCacheKey); ok {
		if snap := v.(*domainSnapshot); time.Since(snap.at) < brandingSnapshotTTL {
			return snap.domains, nil
		}
	}
	var rows []BrandingDomain
	if err := state.DB.Find(&rows).Error; err != nil {
		return nil, err
	}
	domains := make(map[string]*BrandingDomain, len(rows))
	for i := range rows {
		domains[rows[i].Hostname] = &rows[i]
	}
	state.brandingCache.Store(domainCacheKey, &domainSnapshot{at: time.Now(), domains: domains})
	return domains, nil
}

// domainCacheKey shares brandingCache; it cannot collide with a
// broadcaster ID, which is never empty-prefixed like this.
const domainCacheKey = "\x00domains"

// GetBrandingDomain returns the custom domain for hostname, or nil.
func (state *StatefulDB) GetBrandingDomain(hostname string) (*BrandingDomain, error) {
	domains, err := state.brandingDomains()
	if err != nil {
		return nil, err
	}
	return domains[strings.ToLower(hostname)], nil
}

// ListBrandingDomains returns every custom domain, or only ownerDID's when
// it is set, sorted by hostname.
func (state *StatefulDB) ListBrandingDomains(ownerDID string) ([]BrandingDomain, error) {
	domains, err := state.brandingDomains()
	if err != nil {
		return nil, err
	}
	out := []BrandingDomain{}
	for _, d := range domains {
		if ownerDID == "" || d.OwnerDID == ownerDID {
			out = append(out, *d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Hostname < out[j].Hostname })
	return out, nil
}

// PutBrandingDomain adds a custom domain or hands it to a new owner. A new
// owner starts from an empty brand: the previous owner's cached rows go.
func (state *StatefulDB) PutBrandingDomain(hostname, ownerDID string) (*BrandingDomain, error) {
	hostname = strings.ToLower(hostname)
	defer state.brandingCache.Delete(domainCacheKey)
	defer state.forgetBranding("did:web:" + hostname)
	var out BrandingDomain
	err := state.DB.Transaction(func(tx *gorm.DB) error {
		var existing BrandingDomain
		err := tx.Where("hostname = ?", hostname).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			out = BrandingDomain{Hostname: hostname, OwnerDID: ownerDID}
			return tx.Create(&out).Error
		case err != nil:
			return err
		}
		if existing.OwnerDID != ownerDID {
			if err := tx.Where("broadcaster_id = ?", existing.BrandID()).Delete(&BrandingBlob{}).Error; err != nil {
				return err
			}
			existing.OwnerDID = ownerDID
			existing.SyncedAt = nil
			existing.SyncError = ""
			existing.RecordCID = ""
		}
		out = existing
		return tx.Save(&out).Error
	})
	if err != nil {
		return nil, fmt.Errorf("put branding domain %s: %w", hostname, err)
	}
	return &out, nil
}

// DeleteBrandingDomain stops serving hostname as a custom domain and drops
// its cached brand.
func (state *StatefulDB) DeleteBrandingDomain(hostname string) error {
	hostname = strings.ToLower(hostname)
	defer state.brandingCache.Delete(domainCacheKey)
	defer state.forgetBranding("did:web:" + hostname)
	return state.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("broadcaster_id = ?", "did:web:"+hostname).Delete(&BrandingBlob{}).Error; err != nil {
			return err
		}
		return tx.Where("hostname = ?", hostname).Delete(&BrandingDomain{}).Error
	})
}

// MarkBrandingDomainSynced records the outcome of a record pull.
func (state *StatefulDB) MarkBrandingDomainSynced(hostname, recordCID string, syncErr error) error {
	defer state.brandingCache.Delete(domainCacheKey)
	now := time.Now()
	updates := map[string]any{"synced_at": &now, "sync_error": ""}
	if syncErr != nil {
		updates["sync_error"] = syncErr.Error()
	} else {
		updates["record_cid"] = recordCID
	}
	return state.DB.Model(&BrandingDomain{}).Where("hostname = ?", strings.ToLower(hostname)).Updates(updates).Error
}
