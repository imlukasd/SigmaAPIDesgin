package pagination

import (
	"errors"
	"testing"
	"time"
)

func TestNormalizeLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     string
		want      int
		wantError error
	}{
		{name: "default", value: "", want: DefaultLimit},
		{name: "valid", value: "25", want: 25},
		{name: "trim spaces", value: " 10 ", want: 10},
		{name: "cap max", value: "500", want: MaxLimit},
		{name: "reject zero", value: "0", wantError: ErrInvalidLimit},
		{name: "reject negative", value: "-1", wantError: ErrInvalidLimit},
		{name: "reject non integer", value: "abc", wantError: ErrInvalidLimit},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeLimit(tt.value)
			if tt.wantError != nil {
				if !errors.Is(err, tt.wantError) {
					t.Fatalf("expected error %v, got %v", tt.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestEncodeDecodeCursor(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 7, 10, 12, 0, 0, 123, time.UTC)
	cursor := Cursor{
		CreatedAt: createdAt,
		ID:        "018fe2df-4a3a-7c01-9d60-111111111111",
	}

	encoded, err := Encode(cursor)
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}

	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatalf("decode cursor: %v", err)
	}

	if !decoded.CreatedAt.Equal(cursor.CreatedAt) {
		t.Fatalf("expected created_at %s, got %s", cursor.CreatedAt, decoded.CreatedAt)
	}
	if decoded.ID != cursor.ID {
		t.Fatalf("expected id %s, got %s", cursor.ID, decoded.ID)
	}
}

func TestDecodeInvalidCursor(t *testing.T) {
	t.Parallel()

	if _, err := Decode("not-base64"); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestParse(t *testing.T) {
	t.Parallel()

	cursor := Cursor{
		CreatedAt: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
		ID:        "018fe2df-4a3a-7c01-9d60-222222222222",
	}
	encoded, err := Encode(cursor)
	if err != nil {
		t.Fatalf("encode cursor: %v", err)
	}

	request, err := Parse("20", encoded)
	if err != nil {
		t.Fatalf("parse pagination request: %v", err)
	}

	if request.Limit != 20 {
		t.Fatalf("expected limit 20, got %d", request.Limit)
	}
	if request.After == nil {
		t.Fatal("expected cursor")
	}
	if request.After.ID != cursor.ID {
		t.Fatalf("expected cursor id %s, got %s", cursor.ID, request.After.ID)
	}
}
