package devicedetector_test

import (
	"fmt"

	dd "github.com/skalibog/device-detector-go"
	"github.com/skalibog/device-detector-go/parser"
)

func ExampleNew() {
	detector, err := dd.New()
	if err != nil {
		panic(err)
	}

	info, err := detector.Parse("Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Mobile/15E148 Safari/604.1")
	if err != nil {
		panic(err)
	}

	fmt.Println(info.Client().Name, info.Client().Version)
	fmt.Println(info.OS().Name, info.OS().Version)
	fmt.Println(info.DeviceName(), info.Brand())
	// Output:
	// Mobile Safari 17.4
	// iOS 17.4
	// smartphone Apple
}

func ExampleDeviceDetector_Parse_bot() {
	detector, err := dd.New()
	if err != nil {
		panic(err)
	}

	info, err := detector.Parse("Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)")
	if err != nil {
		panic(err)
	}

	fmt.Println(info.IsBot())
	fmt.Println(info.Bot().Name, "|", info.Bot().Category)
	// Output:
	// true
	// Googlebot | Search bot
}

func ExampleWithVersionTruncation() {
	detector, err := dd.New(dd.WithVersionTruncation(parser.VersionTruncationNone))
	if err != nil {
		panic(err)
	}

	info, err := detector.Parse("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.6099.109 Safari/537.36")
	if err != nil {
		panic(err)
	}

	fmt.Println(info.Client().Name, info.Client().Version)
	// Output:
	// Chrome 120.0.6099.109
}
