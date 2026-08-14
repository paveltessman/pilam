package ids_test

import (
	"sort"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/paveltessman/pilam/internal/platform/ids"
)

func generate(g ids.Generator, n int) []ids.ID {
	out := make([]ids.ID, n)
	for i := range out {
		out[i] = g.New()
	}
	return out
}

func TestDeterministicRepeatsItselfForTheSameSeed(t *testing.T) {
	first := generate(ids.NewDeterministic(42), 20)
	second := generate(ids.NewDeterministic(42), 20)

	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("identifier %d differs between runs: %s and %s", i, first[i], second[i])
		}
	}
}

func TestDeterministicDiffersBySeed(t *testing.T) {
	a := generate(ids.NewDeterministic(1), 20)
	b := generate(ids.NewDeterministic(2), 20)

	for i := range a {
		if a[i] == b[i] {
			t.Errorf("seeds 1 and 2 agree on identifier %d: %s", i, a[i])
		}
	}
}

func TestDeterministicOutputIsStableAcrossBuilds(t *testing.T) {
	want := []string{
		"016f5e66-e800-70f3-baad-cc279ed76e43",
		"016f5e66-e801-77b9-a317-0d14b82b2739",
		"016f5e66-e802-7646-9af1-55a02d3fa7b6",
	}

	for i, id := range generate(ids.NewDeterministic(1), len(want)) {
		if id.String() != want[i] {
			t.Errorf("identifier %d = %s, want %s", i, id, want[i])
		}
	}
}

func TestDeterministicProducesValidVersion7Identifiers(t *testing.T) {
	for i, id := range generate(ids.NewDeterministic(7), 100) {
		if id == ids.Nil {
			t.Fatalf("identifier %d is the nil identifier", i)
		}
		if id.Version() != 7 {
			t.Errorf("identifier %d has version %s, want 7", i, id.Version())
		}
		if id.Variant() != uuid.RFC4122 {
			t.Errorf("identifier %d has variant %s, want %s", i, id.Variant(), uuid.RFC4122)
		}
		// Parse is what a URL round trip goes through, so seeded identifiers
		// have to survive it.
		if _, err := ids.Parse(id.String()); err != nil {
			t.Errorf("identifier %d does not parse back: %v", i, err)
		}
	}
}

func TestDeterministicIdentifiersAreDistinctAndOrdered(t *testing.T) {
	const n = 1000

	got := generate(ids.NewDeterministic(3), n)

	text := make([]string, n)
	seen := make(map[ids.ID]struct{}, n)
	for i, id := range got {
		if _, dup := seen[id]; dup {
			t.Fatalf("identifier %d repeats %s", i, id)
		}
		seen[id] = struct{}{}
		text[i] = id.String()
	}

	if !sort.StringsAreSorted(text) {
		t.Error("deterministic identifiers do not sort by generation order")
	}
}

// Run under -race: services share one generator and are not single-goroutine.
// Order across goroutines is not reproducible and is not promised; each
// identifier still being drawn whole from the one stream is.
func TestDeterministicIsSafeForConcurrentUse(t *testing.T) {
	const goroutines, each = 8, 250

	g := ids.NewDeterministic(11)

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		all = make(map[ids.ID]struct{}, goroutines*each)
	)

	for range goroutines {
		wg.Add(1)
		wg.Go(func() {
			defer wg.Done()
			got := generate(g, each)
			mu.Lock()
			defer mu.Unlock()
			for _, id := range got {
				all[id] = struct{}{}
			}
		})
	}
	wg.Wait()

	if len(all) != goroutines*each {
		t.Errorf("got %d distinct identifiers, want %d", len(all), goroutines*each)
	}
}
