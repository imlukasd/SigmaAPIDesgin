package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultLimit = 50
	MaxLimit     = 100
)

var (
	ErrInvalidCursor = errors.New("invalid pagination cursor")
	ErrInvalidLimit  = errors.New("invalid pagination limit")
)

type Cursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        string    `json:"id"`
}

type Request struct {
	Limit int
	After *Cursor
}

type PageInfo struct {
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

func Parse(limitValue string, afterValue string) (Request, error) {
	limit, err := NormalizeLimit(limitValue)
	if err != nil {
		return Request{}, err
	}

	request := Request{Limit: limit}
	if strings.TrimSpace(afterValue) == "" {
		return request, nil
	}

	cursor, err := Decode(afterValue)
	if err != nil {
		return Request{}, err
	}
	request.After = &cursor

	return request, nil
}

func NormalizeLimit(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultLimit, nil
	}

	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 {
		return 0, ErrInvalidLimit
	}
	if limit > MaxLimit {
		return MaxLimit, nil
	}

	return limit, nil
}

func Encode(cursor Cursor) (string, error) {
	if err := validateCursor(cursor); err != nil {
		return "", err
	}

	payload, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("marshal pagination cursor: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func Decode(value string) (Cursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Cursor{}, ErrInvalidCursor
	}

	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}

	var cursor Cursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return Cursor{}, ErrInvalidCursor
	}
	if err := validateCursor(cursor); err != nil {
		return Cursor{}, err
	}

	return cursor, nil
}

func NewPageInfo(hasMore bool, cursor Cursor) (PageInfo, error) {
	if !hasMore {
		return PageInfo{HasMore: false}, nil
	}

	nextCursor, err := Encode(cursor)
	if err != nil {
		return PageInfo{}, err
	}

	return PageInfo{
		HasMore:    true,
		NextCursor: nextCursor,
	}, nil
}

func validateCursor(cursor Cursor) error {
	if cursor.CreatedAt.IsZero() || strings.TrimSpace(cursor.ID) == "" {
		return ErrInvalidCursor
	}
	return nil
}
