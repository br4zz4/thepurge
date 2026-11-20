package docker

import "testing"

func TestPruneKindString(t *testing.T) {
	cases := map[PruneKind]string{
		PruneDangling: "image prune",
		PruneBuilder:  "builder prune",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", k, got, want)
		}
	}
}
