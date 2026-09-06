package storage

import (
	"bytes"
	"testing"
)

func TestMergeRecords_CaseInsensitiveDedup(t *testing.T) {
	cases := []struct {
		desc        string
		localName   string
		localMTime  int64
		remoteName  string
		remoteMTime int64
		wantPass    string
	}{
		{"remote newer wins", "alice", 100, "ALICE", 200, "fromB"},
		{"local newer wins", "ALICE", 200, "alice", 100, "fromA"},
		{"mtime tie: remote wins", "alice", 100, "ALICE", 100, "fromB"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			local := []Record{
				{Name: tc.localName, Username: "u", Password: []byte("fromA"), MTime: tc.localMTime},
			}
			remote := []Record{
				{Name: tc.remoteName, Username: "u", Password: []byte("fromB"), MTime: tc.remoteMTime},
			}

			merged, _ := mergeRecords(nil, local, remote)

			if len(merged) != 1 {
				t.Fatalf("expected 1 record after case-insensitive dedup, got %d: %+v", len(merged), merged)
			}
			if !bytes.Equal(merged[0].Password, []byte(tc.wantPass)) {
				t.Fatalf("expected %q to win; got %q", tc.wantPass, merged[0].Password)
			}
		})
	}
}
