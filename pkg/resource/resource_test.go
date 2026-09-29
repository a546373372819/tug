package resource

import "testing"

func p[T any](v T) *T { return &v }

func TestApplySet(t *testing.T) {
	base := ResourceSet{CPU: 1, Memory: 100, IOWeight: 500}
	got := base.ApplySet(Patch{CPU: p(2.0)})
	if got.CPU != 2 || got.Memory != 100 || got.IOWeight != 500 {
		t.Fatalf("ApplySet overlaid too much: %+v", got)
	}
}

func TestApplyAdd(t *testing.T) {
	base := ResourceSet{CPU: 1, Memory: 100}
	got := base.ApplyAdd(Patch{CPU: p(0.5), Memory: p[int64](50)})
	if got.CPU != 1.5 || got.Memory != 150 {
		t.Fatalf("ApplyAdd wrong: %+v", got)
	}
}

func TestApplyRemoveTruncatesAtZero(t *testing.T) {
	base := ResourceSet{CPU: 1, Memory: 100}
	got := base.ApplyRemove(Patch{CPU: p(5.0), Memory: p[int64](200)})
	if got.CPU != 0 || got.Memory != 0 {
		t.Fatalf("ApplyRemove should truncate at 0: %+v", got)
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		r       ResourceSet
		wantErr bool
	}{
		{"ok", ResourceSet{CPU: 1, Memory: 100 << 20, MemorySwap: 200 << 20}, false},
		{"neg cpu", ResourceSet{CPU: -1}, true},
		{"swap below mem", ResourceSet{Memory: 200, MemorySwap: 100}, true},
		{"io out of range", ResourceSet{IOWeight: 5}, true},
		{"io zero ok", ResourceSet{IOWeight: 0}, false},
		{"reservation above mem", ResourceSet{Memory: 100, MemoryReservation: 200}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.r.Validate()
			if (err != nil) != c.wantErr {
				t.Errorf("Validate(%+v) err=%v, wantErr=%v", c.r, err, c.wantErr)
			}
		})
	}
}
