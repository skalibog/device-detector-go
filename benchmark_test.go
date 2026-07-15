package devicedetector

import "testing"

var benchUAs = []string{
	"Mozilla/5.0 (Linux; Android 7.0; 5061 Build/NRD90M; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/59.0.3071.125 Mobile Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
	"Mozilla/5.0 (SMART-TV; Linux; Tizen 6.0) AppleWebKit/537.36 (KHTML, like Gecko) 76.0.3809.146/6.0 TV Safari/537.36",
	"Dalvik/2.1.0 (Linux; U; Android 11; SM-A125F Build/RP1A.200720.012)",
}

func BenchmarkParse(b *testing.B) {
	detector, err := New()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := detector.Parse(benchUAs[i%len(benchUAs)]); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseParallel(b *testing.B) {
	detector, err := New()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if _, err := detector.Parse(benchUAs[i%len(benchUAs)]); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}
