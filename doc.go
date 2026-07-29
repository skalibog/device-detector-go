// Package devicedetector detects browsers, operating systems, devices, and
// bots from User-Agent strings. It is a native Go port of the matomo
// device-detector library, translated from the PHP sources and validated
// against the complete upstream test corpus (36,333 fixture entries,
// bit-identical output).
//
// # Usage
//
// Construct one detector and share it; construction parses and compiles the
// regex database, so it is expensive, while [DeviceDetector.Parse] is safe
// for concurrent use:
//
//	detector, err := devicedetector.New() // embedded regex database
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	info, err := detector.Parse(userAgent)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	if info.IsBot() {
//		fmt.Println(info.Bot().Name)
//	} else {
//		fmt.Println(info.Client().Name, info.OS().Name, info.DeviceName())
//	}
//
// [New] uses the regex database embedded in the binary. [NewFromDir] and
// [NewFromFS] load an external database instead, which allows updating
// detection data without recompiling.
//
// # Client Hints
//
// [DeviceDetector.ParseWithHints] refines detection with HTTP Client Hints
// (the Sec-CH-UA-* and X-Requested-With headers). Build a [ClientHints] from an
// incoming request with [NewClientHintsFromHeaders], or from the structured
// navigator.userAgentData values with [NewClientHintsFromMap]:
//
//	hints := devicedetector.NewClientHintsFromHeaders(request.Header)
//	info, err := detector.ParseWithHints(request.UserAgent(), hints)
//
// [DeviceDetector.Parse] is exactly ParseWithHints with nil hints.
//
// # Versions
//
// By default reported versions are truncated to minor precision ("17.4"),
// matching the upstream library. Pass [WithVersionTruncation] with a
// [VersionTruncation] constant to change that.
//
// # Detection data
//
// The regex database is taken verbatim from the upstream
// matomo/device-detector project and is licensed LGPL-3.0-or-later, as is
// this package. See the repository README for provenance details and the
// database update workflow.
//
// The detection stages live under internal/ and are not part of the public
// API; everything a caller needs is in this package.
package devicedetector
