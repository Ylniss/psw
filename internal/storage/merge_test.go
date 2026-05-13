package storage

import (
	"bytes"
	"testing"
)

func TestMergeRecords_CaseInsensitiveDedup_RemoteNewerWins(t *testing.T) {
	fork := []Record{}
	local := []Record{
		{Name: "alice", Username: "u", Password: []byte("fromA"), MTime: 100},
	}
	remote := []Record{
		{Name: "ALICE", Username: "u", Password: []byte("fromB"), MTime: 200},
	}

	merged, _ := mergeRecords(fork, local, remote)

	if len(merged) != 1 {
		t.Fatalf("expected 1 record after case-insensitive dedup, got %d: %+v", len(merged), merged)
	}
	if !bytes.Equal(merged[0].Password, []byte("fromB")) {
		t.Fatalf("expected remote's password (newer mtime) to win; got %q", merged[0].Password)
	}
}

func TestMergeRecords_CaseInsensitiveDedup_LocalNewerWins(t *testing.T) {
	fork := []Record{}
	local := []Record{
		{Name: "ALICE", Username: "u", Password: []byte("fromA"), MTime: 200},
	}
	remote := []Record{
		{Name: "alice", Username: "u", Password: []byte("fromB"), MTime: 100},
	}

	merged, _ := mergeRecords(fork, local, remote)

	if len(merged) != 1 {
		t.Fatalf("expected 1 record after case-insensitive dedup, got %d: %+v", len(merged), merged)
	}
	if !bytes.Equal(merged[0].Password, []byte("fromA")) {
		t.Fatalf("expected local's password (newer mtime) to win; got %q", merged[0].Password)
	}
}

func TestMergeRecords_CaseInsensitiveDedup_TieRemoteWins(t *testing.T) {
	fork := []Record{}
	local := []Record{
		{Name: "alice", Username: "u", Password: []byte("fromA"), MTime: 100},
	}
	remote := []Record{
		{Name: "ALICE", Username: "u", Password: []byte("fromB"), MTime: 100},
	}

	merged, _ := mergeRecords(fork, local, remote)

	if len(merged) != 1 {
		t.Fatalf("expected 1 record after case-insensitive dedup, got %d: %+v", len(merged), merged)
	}
	if !bytes.Equal(merged[0].Password, []byte("fromB")) {
		t.Fatalf("expected remote to win on mtime tie; got %q", merged[0].Password)
	}
}
