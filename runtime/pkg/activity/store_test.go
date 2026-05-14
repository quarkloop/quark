package activity

import (
	"encoding/json"
	"testing"
)

func TestStoreBoundsAndCopiesRecords(t *testing.T) {
	store := NewStore(2)
	store.Add("s1", "first", map[string]string{"value": "one"})
	store.Add("s1", "second", map[string]string{"value": "two"})
	store.Add("s1", "third", map[string]string{"value": "three"})

	records := store.List(0)
	if len(records) != 2 {
		t.Fatalf("len(records) = %d, want 2", len(records))
	}
	if records[0].Type != "second" || records[1].Type != "third" {
		t.Fatalf("unexpected records: %+v", records)
	}

	records[0].Data = json.RawMessage(`{"mutated":true}`)
	again := store.List(1)
	if string(again[0].Data) == string(records[0].Data) {
		t.Fatal("store exposed mutable record data")
	}
}

func TestStorePublishesToSubscribers(t *testing.T) {
	store := NewStore(10)
	ch := store.Subscribe()
	defer store.Unsubscribe(ch)

	record := store.Add("s1", "message.user", map[string]string{"role": "user"})
	got := <-ch
	if got.ID != record.ID || got.Type != "message.user" {
		t.Fatalf("subscriber got %+v, want %+v", got, record)
	}
}
