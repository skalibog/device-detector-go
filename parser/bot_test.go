package parser

import (
	"os"
	"testing"
)

func newTestBot(t *testing.T) *Bot {
	t.Helper()

	b, err := NewBot(os.DirFS("../data/regexes"))
	if err != nil {
		t.Fatalf("NewBot: %v", err)
	}

	return b
}

// TestBotParse checks real UA -> bot pairs taken from testdata/fixtures/bots.yml.
func TestBotParse(t *testing.T) {
	b := newTestBot(t)

	tests := []struct {
		name string
		ua   string
		want BotResult
	}{
		{
			name: "360 monitoring",
			ua:   "monitoring360bot/1.1",
			want: BotResult{Name: "360 Monitoring", Category: "Site Monitor", URL: "https://www.360monitoring.io", Producer: BotProducer{Name: "Plesk International GmbH", URL: "https://www.plesk.com"}},
		},
		{
			name: "bitlybot",
			ua:   "bitlybot/3.0",
			want: BotResult{Name: "BitlyBot", Category: "Crawler", URL: "https://bitly.com", Producer: BotProducer{Name: "Bitly, Inc.", URL: "https://bitly.com"}},
		},
		{
			name: "flipboard proxy",
			ua:   "Mozilla/5.0 (compatible; FlipboardProxy/1.2; +http://flipboard.com/browserproxy)",
			want: BotResult{Name: "Flipboard", Category: "Feed Fetcher", URL: "http://flipboard.com/browserproxy", Producer: BotProducer{Name: "Flipboard", URL: "http://flipboard.com/"}},
		},
		{
			name: "rogerbot",
			ua:   "Mozilla/5.0 (compatible; rogerBot/1.0; UrlCrawler; http://www.seomoz.org/dp/rogerbot)",
			want: BotResult{Name: "Rogerbot", Category: "Crawler", URL: "http://moz.com/help/pro/what-is-rogerbot-", Producer: BotProducer{Name: "SEOmoz, Inc.", URL: "http://moz.com/"}},
		},
		{
			name: "wordpress",
			ua:   "WordPress/4.7.2; https://example.com",
			want: BotResult{Name: "WordPress", Category: "Service Agent", URL: "https://wordpress.org/", Producer: BotProducer{Name: "Wordpress.org", URL: "https://wordpress.org/"}},
		},
		{
			name: "iframely",
			ua:   "Iframely/1.6.1 (+https://metadata.xayn.com)",
			want: BotResult{Name: "Iframely", Category: "Crawler", URL: "https://iframely.com/", Producer: BotProducer{Name: "Itteco Software, Corp.", URL: "https://iframely.com/"}},
		},
		{
			name: "ip-guide crawler no producer name",
			ua:   "IP-Guide.com Crawler/1.0 (https://ip-guide.com)",
			want: BotResult{Name: "IP-Guide Crawler", Category: "Crawler", URL: "", Producer: BotProducer{Name: "", URL: "https://ip-guide.com"}},
		},
		{
			name: "expanse security checker",
			ua:   "Expanse indexes the network perimeters of our customers. If you have any questions or concerns, please reach out to: scaninfo@expanseinc.com",
			want: BotResult{Name: "Expanse", Category: "Security Checker", URL: "https://expanse.co/", Producer: BotProducer{Name: "Expanse Inc.", URL: "https://expanse.co/"}},
		},
		{
			name: "googlebot",
			ua:   "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
			want: BotResult{Name: "Googlebot", Category: "Search bot", URL: "https://developers.google.com/search/docs/crawling-indexing/overview-google-crawlers", Producer: BotProducer{Name: "Google Inc.", URL: "https://www.google.com/"}},
		},
		{
			name: "generic bot without details",
			ua:   "nvd0rz",
			want: BotResult{Name: "Generic Bot", Category: "", URL: "", Producer: BotProducer{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := b.Parse(tt.ua)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			if got == nil {
				t.Fatalf("Parse(%q) = nil, want %+v", tt.ua, tt.want)
			}

			if *got != tt.want {
				t.Errorf("Parse(%q)\n got  %+v\n want %+v", tt.ua, *got, tt.want)
			}
		})
	}
}

// TestBotParseNotABot verifies that ordinary browser user agents are not bots.
func TestBotParseNotABot(t *testing.T) {
	b := newTestBot(t)

	uas := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.0 Mobile/15E148 Safari/604.1",
	}

	for _, ua := range uas {
		got, err := b.Parse(ua)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}

		if got != nil {
			t.Errorf("Parse(%q) = %+v, want nil", ua, *got)
		}
	}
}

// TestBotIsBot exercises the discardDetails fast path.
func TestBotIsBot(t *testing.T) {
	b := newTestBot(t)

	tests := []struct {
		ua   string
		want bool
	}{
		{"monitoring360bot/1.1", true},
		{"Googlebot/2.1 (+http://www.google.com/bot.html)", true},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0 Safari/537.36", false},
	}

	for _, tt := range tests {
		got, err := b.IsBot(tt.ua)
		if err != nil {
			t.Fatalf("IsBot: %v", err)
		}

		if got != tt.want {
			t.Errorf("IsBot(%q) = %v, want %v", tt.ua, got, tt.want)
		}
	}
}
