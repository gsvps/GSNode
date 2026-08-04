package probe

import "testing"

func TestParseScamalyticsScore(t *testing.T) {
	// Real page shape: <h1>… Fraud Risk</h1> must NOT yield score=1 from </h1>.
	html := `
<html><body>
<h1>1.1.1.1 Fraud Risk</h1>
<div class="score">0</div>
<p>Fraud Score: 21</p>
<pre>{"ip":"1.1.1.1","score":"21","risk":"low"}</pre>
</body></html>`
	got, ok := parseScamalyticsScore(html)
	if !ok || got != 21 {
		t.Fatalf("parseScamalyticsScore() = %d ok=%v, want 21", got, ok)
	}

	htmlZero := `<h1>8.8.8.8 Fraud Risk</h1><p>Fraud Score: 0</p>`
	got, ok = parseScamalyticsScore(htmlZero)
	if !ok || got != 0 {
		t.Fatalf("zero score = %d ok=%v, want 0", got, ok)
	}

	htmlJSON := `<h1>x Fraud Risk</h1>{"score":"65","risk":"high"}`
	got, ok = parseScamalyticsScore(htmlJSON)
	if !ok || got != 65 {
		t.Fatalf("json score = %d ok=%v, want 65", got, ok)
	}

	// Only the broken heading — must fail, not invent 1.
	if _, ok := parseScamalyticsScore(`<h1>1.2.3.4 Fraud Risk</h1>`); ok {
		t.Fatal("heading-only HTML should not yield a score")
	}
}

func TestParseScamalyticsScoreRejectsOver100(t *testing.T) {
	if _, ok := parseScamalyticsScore(`Fraud Score: 999`); ok {
		t.Fatal("score > 100 should be rejected")
	}
}
