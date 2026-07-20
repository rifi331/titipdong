package orders

import "testing"

func TestNextStatus(t *testing.T) {
	cases := []struct {
		in, want Status
	}{
		{StatusDicari, StatusKetemu},
		{StatusKetemu, StatusDibeli},
		{StatusDibeli, StatusDibayar},
		{StatusDibayar, StatusDiantar},
		{StatusDiantar, StatusDiantar}, // final stays
	}
	for _, c := range cases {
		if got := NextStatus(c.in); got != c.want {
			t.Errorf("NextStatus(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestStatusLabel(t *testing.T) {
	if StatusLabel(StatusKetemu) != "Ketemu" {
		t.Error("label mismatch")
	}
}

func TestPipeline_Order(t *testing.T) {
	want := []Status{StatusDicari, StatusKetemu, StatusDibeli, StatusDibayar, StatusDiantar}
	if len(Pipeline) != len(want) {
		t.Fatalf("pipeline length = %d, want %d", len(Pipeline), len(want))
	}
	for i, s := range Pipeline {
		if s != want[i] {
			t.Errorf("pipeline[%d] = %v, want %v", i, s, want[i])
		}
	}
}
