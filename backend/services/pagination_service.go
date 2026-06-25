package services

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	cursorPrefix    = "cursor:"
)

// PageInfo holds relay-style pagination info
type PageInfo struct {
	HasNextPage     bool    `json:"hasNextPage"`
	HasPreviousPage bool    `json:"hasPreviousPage"`
	StartCursor     *string `json:"startCursor"`
	EndCursor       *string `json:"endCursor"`
}

// EncodeCursor encodes a record ID into an opaque cursor string
func EncodeCursor(id uint) string {
	raw := fmt.Sprintf("%s%d", cursorPrefix, id)
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor decodes a cursor string back to a record ID
// Returns 0 and nil error when cursor is empty
func DecodeCursor(cursor string) (uint, error) {
	if cursor == "" {
		return 0, nil
	}
	b, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor encoding")
	}
	raw := string(b)
	if !strings.HasPrefix(raw, cursorPrefix) {
		return 0, fmt.Errorf("invalid cursor format")
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(raw, cursorPrefix), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cursor value")
	}
	return uint(id), nil
}

// NormalizePaginationArgs validates and normalizes first/last/after/before args.
// Returns (limit, afterID, beforeID, forward, error).
// forward=true means paginating forward (first/after), forward=false means backward (last/before).
func NormalizePaginationArgs(first, last *int, after, before string) (limit int, afterID uint, beforeID uint, forward bool, err error) {
	afterID, err = DecodeCursor(after)
	if err != nil {
		return 0, 0, 0, true, fmt.Errorf("invalid after cursor: %w", err)
	}
	beforeID, err = DecodeCursor(before)
	if err != nil {
		return 0, 0, 0, true, fmt.Errorf("invalid before cursor: %w", err)
	}

	if last != nil && before != "" {
		forward = false
		limit = *last
	} else {
		forward = true
		if first != nil {
			limit = *first
		} else {
			limit = defaultPageSize
		}
	}

	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	return limit, afterID, beforeID, forward, nil
}

// BuildPageInfo constructs PageInfo from a result slice's first/last IDs and whether more pages exist
func BuildPageInfo(hasNext, hasPrev bool, startID, endID uint) PageInfo {
	pi := PageInfo{
		HasNextPage:     hasNext,
		HasPreviousPage: hasPrev,
	}
	if startID > 0 {
		c := EncodeCursor(startID)
		pi.StartCursor = &c
	}
	if endID > 0 {
		c := EncodeCursor(endID)
		pi.EndCursor = &c
	}
	return pi
}
