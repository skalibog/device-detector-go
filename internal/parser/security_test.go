package parser

import (
	"testing"
	"testing/fstest"
	"time"
)

func TestPreMatchEmpty(t *testing.T) {
	if !PreMatchEmpty(CombineRegexes(nil)) {
		t.Error("combined regex of an empty list must be treated as no-match")
	}

	if PreMatchEmpty(CombineRegexes([]string{"Chrome"})) {
		t.Error("combined regex of a non-empty list must not be no-match")
	}
}

// TestEmptyBotListNoFalsePositive is the regression test for the upstream 6.5.1
// fix (PR #8271): a parser built from an empty regex list must not degrade into
// a bare anchor that matches every user agent.
func TestEmptyBotListNoFalsePositive(t *testing.T) {
	bot, err := NewBot(fstest.MapFS{"bots.yml": {Data: []byte("[]\n")}})
	if err != nil {
		t.Fatal(err)
	}

	isBot, err := bot.IsBot("Mozilla/5.0 (compatible; Googlebot/2.1)")
	if err != nil {
		t.Fatal(err)
	}

	if isBot {
		t.Error("empty bot database must never report a bot (empty-regex false positive)")
	}

	res, err := bot.Parse("anything at all")
	if err != nil || res != nil {
		t.Errorf("empty bot database Parse: got (%v, %v), want (nil, nil)", res, err)
	}
}

// TestEmptyIndividualPatternStillMatches documents the deliberate asymmetry:
// an individual empty pattern keeps its catch-all semantics (the database uses
// them as model fallbacks, e.g. Roku's "Digital Video Player"), even though an
// empty preMatchOverall list does not.
func TestEmptyIndividualPatternStillMatches(t *testing.T) {
	m, err := MatchUserAgent("any user agent", "")
	if err != nil {
		t.Fatal(err)
	}

	if m == nil {
		t.Error("an individual empty pattern must match every user agent (catch-all)")
	}
}

func TestMatchTimeoutRoundtrip(t *testing.T) {
	orig := MatchTimeout()
	t.Cleanup(func() { SetMatchTimeout(orig) })

	SetMatchTimeout(250 * time.Millisecond)

	if got := MatchTimeout(); got != 250*time.Millisecond {
		t.Errorf("MatchTimeout = %v, want 250ms", got)
	}

	SetMatchTimeout(-1)

	if got := MatchTimeout(); got != 0 {
		t.Errorf("negative timeout must disable (0), got %v", got)
	}
}
