package devicedetector_test

import (
	"fmt"

	dd "github.com/skalibog/devicedetector"
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
