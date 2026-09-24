package social

import (
	"context"
	"testing"
)

func (m *memoryStore) SaveStealRecords(ctx context.Context, accountKey string, records []StealRecord) error {
	m.stealRecords = records
	return nil
}

func (m *memoryStore) SaveVisitorRecords(ctx context.Context, accountKey string, records []VisitorRecord) error {
	m.visitors = records
	return nil
}

func TestExportSelectedSocialGroups(t *testing.T) {
	store := &memoryStore{
		rules: FriendRules{Blacklist: []string{"10002"}, Whitelist: []string{"10001"}},
		stealRecords: []StealRecord{
			{ID: "s1", GID: 10001, DisplayName: "A", OccurredAt: "2026-07-08T08:00:00Z", StealCount: 1},
		},
		dogGuard: DogGuardState{Results: []DogGuardRow{{GID: 10001, HasGuardDog: true}}},
	}
	service := NewService(store, nil, Options{AccountKey: "account-a"})

	payload := service.Export(context.Background(), ExportRequest{Groups: []string{"rules", "steal_records", "dog_guard"}})

	if payload.Version != 1 || payload.AccountKey != "account-a" {
		t.Fatalf("payload metadata = %#v", payload)
	}
	if payload.Rules == nil || payload.Rules.Blacklist[0] != "10002" {
		t.Fatalf("rules not exported: %#v", payload.Rules)
	}
	if len(payload.StealRecords) != 1 || payload.StealRecords[0].ID != "s1" {
		t.Fatalf("steal records not exported: %#v", payload.StealRecords)
	}
	if payload.DogGuard == nil || payload.DogGuard.HasGuardDogCount != 1 {
		t.Fatalf("dog guard not exported: %#v", payload.DogGuard)
	}
}

func TestImportRejectsUnknownVersion(t *testing.T) {
	service := NewService(&memoryStore{}, nil, Options{AccountKey: "account-a"})

	result := service.Import(context.Background(), ImportExportPayload{Version: 99, Groups: []string{"rules"}})

	if result.OK || result.Status != StatusFailed {
		t.Fatalf("result = %#v", result)
	}
}

func TestImportSelectedSocialGroups(t *testing.T) {
	store := &memoryStore{}
	service := NewService(store, nil, Options{AccountKey: "account-a"})
	rules := FriendRules{Blacklist: []string{"10002"}}
	dogGuard := DogGuardState{Results: []DogGuardRow{{GID: 10001, HasGuardDog: true}}}

	result := service.Import(context.Background(), ImportExportPayload{
		Version:  1,
		Groups:   []string{"rules", "dog_guard"},
		Rules:    &rules,
		DogGuard: &dogGuard,
	})

	if !result.OK || store.rules.Blacklist[0] != "10002" || store.dogGuard.HasGuardDogCount != 1 {
		t.Fatalf("import result = %#v store = %#v", result, store)
	}
}
