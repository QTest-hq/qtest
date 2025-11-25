// Package validation provides input validation utilities for the QTest API.
package validation

import (
	"fmt"
	"strconv"
)

const (
	// DefaultLimit is the default number of items per page
	DefaultLimit = 20

	// MaxLimit is the maximum number of items per page
	MaxLimit = 100

	// MaxOffset is the maximum offset to prevent deep pagination DoS attacks
	MaxOffset = 10000
)

// PaginationParams holds validated pagination parameters
type PaginationParams struct {
	Limit  int
	Offset int
}

// ValidatePagination validates and normalizes pagination parameters.
// Returns an error if the parameters are invalid.
func ValidatePagination(limitStr, offsetStr string) (*PaginationParams, error) {
	params := &PaginationParams{
		Limit:  DefaultLimit,
		Offset: 0,
	}

	// Parse limit
	if limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return nil, fmt.Errorf("invalid limit: must be a number")
		}
		if limit < 0 {
			return nil, fmt.Errorf("invalid limit: must be non-negative")
		}
		if limit > MaxLimit {
			limit = MaxLimit // Clamp to max instead of erroring
		}
		if limit == 0 {
			limit = DefaultLimit
		}
		params.Limit = limit
	}

	// Parse offset
	if offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			return nil, fmt.Errorf("invalid offset: must be a number")
		}
		if offset < 0 {
			return nil, fmt.Errorf("invalid offset: must be non-negative")
		}
		if offset > MaxOffset {
			return nil, fmt.Errorf("invalid offset: maximum offset is %d", MaxOffset)
		}
		params.Offset = offset
	}

	return params, nil
}

// ValidatePageNumber validates page number and converts to offset.
// Page numbers start at 1.
func ValidatePageNumber(pageStr, limitStr string) (*PaginationParams, error) {
	params := &PaginationParams{
		Limit:  DefaultLimit,
		Offset: 0,
	}

	// Parse limit first
	if limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return nil, fmt.Errorf("invalid limit: must be a number")
		}
		if limit < 0 {
			return nil, fmt.Errorf("invalid limit: must be non-negative")
		}
		if limit > MaxLimit {
			limit = MaxLimit
		}
		if limit == 0 {
			limit = DefaultLimit
		}
		params.Limit = limit
	}

	// Parse page number
	if pageStr != "" {
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			return nil, fmt.Errorf("invalid page: must be a number")
		}
		if page < 1 {
			return nil, fmt.Errorf("invalid page: must be >= 1")
		}

		// Convert page to offset
		offset := (page - 1) * params.Limit
		if offset > MaxOffset {
			return nil, fmt.Errorf("page number too high: maximum offset is %d", MaxOffset)
		}
		params.Offset = offset
	}

	return params, nil
}
