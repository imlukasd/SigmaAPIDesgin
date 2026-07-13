package id

import "testing"

func TestNewUUID(t *testing.T) {
	t.Parallel()

	id, err := NewUUID()
	if err != nil {
		t.Fatalf("new uuid: %v", err)
	}

	if len(id) != 36 {
		t.Fatalf("expected uuid length 36, got %d", len(id))
	}
	if id[14] != '4' {
		t.Fatalf("expected version 4 uuid, got %s", id)
	}
}
