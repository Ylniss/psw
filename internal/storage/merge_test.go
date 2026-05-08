package storage

import (
	"reflect"
	"sort"
	"testing"
)

func rec(name, user, pass string, mtime int64) Record {
	return Record{Name: name, Username: user, Password: pass, MTime: mtime}
}

func TestMergeRecords(t *testing.T) {
	tests := []struct {
		name           string
		fork           []Record
		local          []Record
		remote         []Record
		wantRecords    []Record
		wantActions    []mergeAction
		wantNamesByAct map[mergeAction][]string
	}{
		{
			name:        "all empty",
			wantRecords: nil,
		},
		{
			name:        "F0 L1 R0: local-added since fork",
			local:       []Record{rec("alice", "u", "p", 5)},
			wantRecords: []Record{rec("alice", "u", "p", 5)},
		},
		{
			name:           "F0 L0 R1: remote-added since fork",
			remote:         []Record{rec("alice", "u", "p", 5)},
			wantRecords:    []Record{rec("alice", "u", "p", 5)},
			wantNamesByAct: map[mergeAction][]string{actionAddedFromRemote: {"alice"}},
		},
		{
			name:           "F0 L1 R1: independent adds, remote newer",
			local:          []Record{rec("alice", "u", "L", 5)},
			remote:         []Record{rec("alice", "u", "R", 7)},
			wantRecords:    []Record{rec("alice", "u", "R", 7)},
			wantNamesByAct: map[mergeAction][]string{actionReplacedFromRemote: {"alice"}},
		},
		{
			name:           "F0 L1 R1: independent adds, local newer",
			local:          []Record{rec("alice", "u", "L", 9)},
			remote:         []Record{rec("alice", "u", "R", 5)},
			wantRecords:    []Record{rec("alice", "u", "L", 9)},
			wantNamesByAct: map[mergeAction][]string{actionKeptLocalNewer: {"alice"}},
		},
		{
			name:           "F0 L1 R1: independent adds, mtime tie → remote wins",
			local:          []Record{rec("alice", "u", "L", 5)},
			remote:         []Record{rec("alice", "u", "R", 5)},
			wantRecords:    []Record{rec("alice", "u", "R", 5)},
			wantNamesByAct: map[mergeAction][]string{actionReplacedFromRemote: {"alice"}},
		},
		{
			name:           "F1 L1 R0: remote removed, local untouched → drop",
			fork:           []Record{rec("alice", "u", "p", 5)},
			local:          []Record{rec("alice", "u", "p", 5)},
			wantRecords:    nil,
			wantNamesByAct: map[mergeAction][]string{actionDroppedByRemote: {"alice"}},
		},
		{
			name:           "F1 L1 R0: remote removed, local modified after fork → keep local",
			fork:           []Record{rec("alice", "u", "p", 5)},
			local:          []Record{rec("alice", "u", "newp", 9)},
			wantRecords:    []Record{rec("alice", "u", "newp", 9)},
			wantNamesByAct: map[mergeAction][]string{actionKeptLocalOverRemoval: {"alice"}},
		},
		{
			name:        "F1 L0 R1: local removed, remote untouched → drop (no warning)",
			fork:        []Record{rec("alice", "u", "p", 5)},
			remote:      []Record{rec("alice", "u", "p", 5)},
			wantRecords: nil,
		},
		{
			name:           "F1 L0 R1: local removed, remote modified after fork → take remote",
			fork:           []Record{rec("alice", "u", "p", 5)},
			remote:         []Record{rec("alice", "u", "newp", 9)},
			wantRecords:    []Record{rec("alice", "u", "newp", 9)},
			wantNamesByAct: map[mergeAction][]string{actionAddedFromRemote: {"alice"}},
		},
		{
			name:        "F1 L1 R1: byte-equal → unchanged, no warning",
			fork:        []Record{rec("alice", "u", "p", 5)},
			local:       []Record{rec("alice", "u", "p", 5)},
			remote:      []Record{rec("alice", "u", "p", 5)},
			wantRecords: []Record{rec("alice", "u", "p", 5)},
		},
		{
			name:           "F1 L1 R1: differing content, remote newer",
			fork:           []Record{rec("alice", "u", "p0", 5)},
			local:          []Record{rec("alice", "u", "L", 7)},
			remote:         []Record{rec("alice", "u", "R", 9)},
			wantRecords:    []Record{rec("alice", "u", "R", 9)},
			wantNamesByAct: map[mergeAction][]string{actionReplacedFromRemote: {"alice"}},
		},
		{
			name:           "F1 L1 R1: differing content, local newer",
			fork:           []Record{rec("alice", "u", "p0", 5)},
			local:          []Record{rec("alice", "u", "L", 9)},
			remote:         []Record{rec("alice", "u", "R", 7)},
			wantRecords:    []Record{rec("alice", "u", "L", 9)},
			wantNamesByAct: map[mergeAction][]string{actionKeptLocalNewer: {"alice"}},
		},
		{
			name:        "F1 L0 R0: both removed → drop, no warning",
			fork:        []Record{rec("alice", "u", "p", 5)},
			wantRecords: nil,
		},
		{
			name:           "case-insensitive name matching: remote wins on tie, preserves remote casing",
			local:          []Record{rec("alice", "u", "L", 5)},
			remote:         []Record{rec("ALICE", "u", "R", 5)},
			wantRecords:    []Record{rec("ALICE", "u", "R", 5)},
			wantNamesByAct: map[mergeAction][]string{actionReplacedFromRemote: {"ALICE"}},
		},
		{
			name: "multi-record disjoint adds + content reconcile",
			fork: []Record{rec("shared", "u", "p0", 5)},
			local: []Record{
				rec("local-only", "u", "p", 6),
				rec("shared", "u", "L", 9),
			},
			remote: []Record{
				rec("remote-only", "u", "p", 7),
				rec("shared", "u", "R", 7),
			},
			wantRecords: []Record{
				rec("local-only", "u", "p", 6),
				rec("remote-only", "u", "p", 7),
				rec("shared", "u", "L", 9),
			},
			wantNamesByAct: map[mergeAction][]string{
				actionAddedFromRemote: {"remote-only"},
				actionKeptLocalNewer:  {"shared"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged, summary := mergeRecords(tt.fork, tt.local, tt.remote)
			if !reflect.DeepEqual(merged, tt.wantRecords) {
				t.Errorf("records mismatch:\n got: %+v\nwant: %+v", merged, tt.wantRecords)
			}
			gotByAct := map[mergeAction][]string{}
			for _, c := range summary.changes {
				gotByAct[c.action] = append(gotByAct[c.action], c.name)
			}
			for _, names := range gotByAct {
				sort.Strings(names)
			}
			for _, names := range tt.wantNamesByAct {
				sort.Strings(names)
			}
			want := tt.wantNamesByAct
			if want == nil {
				want = map[mergeAction][]string{}
			}
			if !reflect.DeepEqual(gotByAct, want) {
				t.Errorf("summary actions mismatch:\n got: %+v\nwant: %+v", gotByAct, want)
			}
		})
	}
}

func TestPickByMTime_RemoteWinsTie(t *testing.T) {
	l := rec("a", "u", "L", 5)
	r := rec("a", "u", "R", 5)
	got, action := pickByMTime(l, r)
	if got.Password != "R" {
		t.Errorf("tie should prefer remote, got %s", got.Password)
	}
	if action != actionReplacedFromRemote {
		t.Errorf("tie action: got %d want %d", action, actionReplacedFromRemote)
	}
}

func TestRecordsEqual_IgnoresMTime(t *testing.T) {
	a := rec("x", "u", "p", 1)
	b := rec("x", "u", "p", 999)
	if !recordsEqual(a, b) {
		t.Error("recordsEqual should ignore MTime")
	}
}
