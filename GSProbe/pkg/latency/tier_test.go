package latency

import "testing"

func TestTierValues(t *testing.T) {
	cases := map[string]struct {
		ms   float64
		text string
		want string
	}{
		"excellent": {50, "50 ms", "excellent"},
		"good-low":  {51, "51 ms", "good"},
		"good-high": {100, "100 ms", "good"},
		"fair-low":  {101, "101 ms", "fair"},
		"fair-high": {200, "200 ms", "fair"},
		"medium":    {220, "220 ms", "medium"},
		"high":      {300, "300 ms", "high"},
		"timeout":   {0, "超时", "timeout"},
		"unknown":   {0, "—", "unknown"},
	}
	for name, tc := range cases {
		if got := Tier(tc.ms, tc.text); got != tc.want {
			t.Fatalf("%s: Tier(%v, %q) = %q, want %q", name, tc.ms, tc.text, got, tc.want)
		}
	}
}

func TestScoreFromLatency(t *testing.T) {
	if ScoreFromLatency(40) != 100 || ScoreFromLatency(80) != 95 || ScoreFromLatency(150) != 85 || ScoreFromLatency(230) != 70 || ScoreFromLatency(400) != 55 {
		t.Fatal("score tiers mismatch")
	}
}
