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
// # Versions
//
// By default reported versions are truncated to minor precision ("17.4"),
// matching the upstream library. Pass [WithVersionTruncation] with a
// parser.VersionTruncation* constant to change that.
//
// # Detection data
//
// The regex database is taken verbatim from the upstream
// matomo/device-detector project and is licensed LGPL-3.0-or-later, as is
// this package. See the repository README for provenance details and the
// database update workflow.
//
// The subpackages parser, parser/client and parser/device contain the
// individual detection stages; most applications only need this root
// package.
package devicedetector
