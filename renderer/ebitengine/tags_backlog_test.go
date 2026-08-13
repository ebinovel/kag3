package ebitengine

import "testing"

// TestTrimBacklogKeepsMostRecent is the regression test for #14:
// Config.MaxBackLogNum was declared and defaulted (50) but never actually
// enforced — recordBacklog appended forever. trimBacklog now caps it,
// keeping the newest entries.
func TestTrimBacklogKeepsMostRecent(t *testing.T) {
	var backlog []backlogEntry
	for i := 0; i < 5; i++ {
		backlog = append(backlog, backlogEntry{Text: string(rune('a' + i))})
	}
	got := trimBacklog(backlog, 3)
	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	want := []string{"c", "d", "e"}
	for i, w := range want {
		if got[i].Text != w {
			t.Errorf("got[%d].Text = %q, want %q", i, got[i].Text, w)
		}
	}
}

// TestTrimBacklogUnlimitedWhenMaxNotPositive confirms max<=0 (either the
// zero value or an explicit config choice) leaves backlog untouched, rather
// than trimming it to nothing.
func TestTrimBacklogUnlimitedWhenMaxNotPositive(t *testing.T) {
	var backlog []backlogEntry
	for i := 0; i < 5; i++ {
		backlog = append(backlog, backlogEntry{Text: string(rune('a' + i))})
	}
	for _, max := range []int{0, -1} {
		got := trimBacklog(backlog, max)
		if len(got) != 5 {
			t.Errorf("max=%d: len(got) = %d, want 5 (unlimited)", max, len(got))
		}
	}
}

// TestRecordBacklogTrimsToConfigMax exercises the actual call site:
// recordBacklog reads r.manager.Config.MaxBackLogNum on every call, not
// just trimBacklog in isolation.
func TestRecordBacklogTrimsToConfigMax(t *testing.T) {
	origBacklog := backlog
	t.Cleanup(func() { backlog = origBacklog })
	backlog = nil

	r := newTestRenderer()
	r.manager.Config.MaxBackLogNum = 3

	for i := 0; i < 5; i++ {
		r.texts = map[int][]Text{0: {{Text: string(rune('a' + i))}}}
		recordBacklog(r)
	}
	if len(backlog) != 3 {
		t.Fatalf("len(backlog) = %d, want 3", len(backlog))
	}
	want := []string{"c", "d", "e"}
	for i, w := range want {
		if backlog[i].Text != w {
			t.Errorf("backlog[%d].Text = %q, want %q", i, backlog[i].Text, w)
		}
	}
}
