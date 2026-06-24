package models

import "time"

// SearchFilter is the normalized set of filters accepted by asset search.
// Metadata values are matched exactly; callers may provide several values for
// the same field and all supplied fields must match.
type SearchFilter struct {
	AssetTypes  []string          `form:"asset_type" json:"asset_types,omitempty"`
	MinPrice    *int64            `form:"min_price" json:"min_price,omitempty"`
	MaxPrice    *int64            `form:"max_price" json:"max_price,omitempty"`
	Locations   []string          `form:"location" json:"locations,omitempty"`
	CreatedFrom *time.Time        `form:"created_from" json:"created_from,omitempty"`
	CreatedTo   *time.Time        `form:"created_to" json:"created_to,omitempty"`
	Verified    *bool             `form:"verified" json:"verified,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}
