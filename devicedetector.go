package devicedetector

import (
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/dlclark/regexp2"

	"github.com/skalibog/device-detector-go/parser"
	"github.com/skalibog/device-detector-go/parser/client"
	"github.com/skalibog/device-detector-go/parser/device"
)

// Unknown is the value PHP DeviceDetector reports for unknown attributes.
const Unknown = "UNK"

// DeviceDetector parses user agent strings. It is immutable after
// construction and safe for concurrent use.
type DeviceDetector struct {
	bot           *parser.Bot
	os            *parser.OS
	vendor        *parser.VendorFragment
	clientParsers []client.Parser
	deviceParsers []device.Parser

	skipBotDetection bool
	truncation       int
}

// Option configures a DeviceDetector.
type Option func(*DeviceDetector)

// WithVersionTruncation sets how detailed reported versions are
// (parser.VersionTruncation* constants). Default is minor precision,
// matching the PHP library.
func WithVersionTruncation(t int) Option {
	return func(d *DeviceDetector) { d.truncation = t }
}

// WithSkipBotDetection disables bot detection entirely, mirroring
// DeviceDetector::skipBotDetection().
func WithSkipBotDetection() Option {
	return func(d *DeviceDetector) { d.skipBotDetection = true }
}

// New creates a detector backed by the embedded regex database.
func New(opts ...Option) (*DeviceDetector, error) {
	return NewFromFS(EmbeddedRegexes(), opts...)
}

// NewFromDir creates a detector loading the regex database from a directory,
// for out-of-band database updates.
func NewFromDir(dir string, opts ...Option) (*DeviceDetector, error) {
	return NewFromFS(os.DirFS(dir), opts...)
}

// NewFromFS creates a detector loading the regex database from fsys.
func NewFromFS(fsys fs.FS, opts ...Option) (*DeviceDetector, error) {
	d := &DeviceDetector{truncation: parser.VersionTruncationMinor}

	for _, opt := range opts {
		opt(d)
	}

	var err error

	if d.bot, err = parser.NewBot(fsys); err != nil {
		return nil, err
	}

	if d.os, err = parser.NewOS(fsys); err != nil {
		return nil, err
	}

	d.os.SetVersionTruncation(d.truncation)

	if d.vendor, err = parser.NewVendorFragment(fsys); err != nil {
		return nil, err
	}

	if d.clientParsers, err = client.All(fsys); err != nil {
		return nil, err
	}

	for _, p := range d.clientParsers {
		p.SetVersionTruncation(d.truncation)
	}

	if d.deviceParsers, err = device.All(fsys); err != nil {
		return nil, err
	}

	return d, nil
}

// Info holds the outcome of parsing a single user agent.
type Info struct {
	UserAgent string

	bot        *parser.BotResult
	os         *parser.OSResult
	client     *client.Result
	deviceType int
	brand      string
	model      string
}

// IsBot reports whether the UA was identified as a bot.
func (i *Info) IsBot() bool { return i.bot != nil }

// Bot returns bot details, or nil for non-bot traffic.
func (i *Info) Bot() *parser.BotResult { return i.bot }

// OS returns operating system details, or nil when undetected.
func (i *Info) OS() *parser.OSResult { return i.os }

// Client returns client (browser/app/...) details, or nil when undetected.
func (i *Info) Client() *client.Result { return i.client }

// Device returns the detected device type id (device.Type* constants),
// or device.TypeUnknown.
func (i *Info) Device() int { return i.deviceType }

// DeviceName returns the canonical device type name ("smartphone", ...),
// or "" when unknown.
func (i *Info) DeviceName() string {
	if i.deviceType == device.TypeUnknown {
		return ""
	}

	return device.TypeName(i.deviceType)
}

// Brand returns the device brand name, or "".
func (i *Info) Brand() string { return i.brand }

// Model returns the device model, or "".
func (i *Info) Model() string { return i.model }

func (i *Info) osAttr(get func(*parser.OSResult) string) string {
	if i.os == nil {
		return Unknown
	}

	if v := get(i.os); v != "" {
		return v
	}

	return Unknown
}

// IsTouchEnabled mirrors DeviceDetector::isTouchEnabled().
func (i *Info) IsTouchEnabled() bool { return matchUA(i.UserAgent, `Touch`) }

func (i *Info) usesMobileBrowser() bool {
	return i.client != nil && i.client.Type == "browser" &&
		client.IsMobileOnlyBrowser(i.client.Name)
}

// IsDesktop mirrors DeviceDetector::isDesktop(): unknown-type devices running
// a desktop OS with a non-mobile-only browser.
func (i *Info) IsDesktop() bool {
	osName := i.osAttr(func(o *parser.OSResult) string { return o.Name })
	if osName == Unknown {
		return false
	}

	if i.usesMobileBrowser() {
		return false
	}

	return parser.IsDesktopOS(osName)
}

// IsMobile mirrors DeviceDetector::isMobile().
func (i *Info) IsMobile() bool {
	switch i.deviceType {
	case device.TypeFeaturePhone, device.TypeSmartphone, device.TypeTablet,
		device.TypePhablet, device.TypeCamera, device.TypePortableMediaPlayer:
		return true
	case device.TypeTV, device.TypeSmartDisplay, device.TypeConsole:
		return false
	}

	if i.usesMobileBrowser() {
		return true
	}

	if i.os == nil || i.os.Name == "" || i.os.Name == Unknown {
		return false
	}

	return !i.IsBot() && !i.IsDesktop()
}

var hasLetterRe = regexp.MustCompile(`[a-zA-Z]`)

// Parse runs the full detection pipeline on ua, mirroring
// DeviceDetector::parse().
func (d *DeviceDetector) Parse(ua string) (*Info, error) {
	info := &Info{UserAgent: ua, deviceType: device.TypeUnknown}

	if ua == "" || !hasLetterRe.MatchString(ua) {
		return info, nil
	}

	if !d.skipBotDetection {
		bot, err := d.bot.Parse(ua)
		if err != nil {
			return nil, err
		}

		if bot != nil {
			info.bot = bot

			return info, nil
		}
	}

	osResult, err := d.os.Parse(ua)
	if err != nil {
		return nil, err
	}

	info.os = osResult

	for _, p := range d.clientParsers {
		res, err := p.Parse(ua)
		if err != nil {
			return nil, err
		}

		if res != nil {
			info.client = res

			break
		}
	}

	if err := d.parseDevice(info); err != nil {
		return nil, err
	}

	return info, nil
}

// parseDevice mirrors DeviceDetector::parseDevice(), including the long
// post-detection heuristics chain. Order of checks is load-bearing.
func (d *DeviceDetector) parseDevice(info *Info) error {
	ua := info.UserAgent

	for _, p := range d.deviceParsers {
		res, err := p.Parse(ua)
		if err != nil {
			return err
		}

		if res != nil {
			info.deviceType = res.Type
			info.model = res.Model
			info.brand = res.Brand

			break
		}
	}

	if info.brand == "" {
		brand, _, err := d.vendor.Parse(ua)
		if err != nil {
			return err
		}

		info.brand = brand
	}

	osName := info.osAttr(func(o *parser.OSResult) string { return o.Name })
	osFamily := info.osAttr(func(o *parser.OSResult) string { return o.Family })
	osVersion := info.osAttr(func(o *parser.OSResult) string { return o.Version })
	clientName := ""

	if info.client != nil {
		clientName = info.client.Name
	}

	appleOsNames := []string{"iPadOS", "tvOS", "watchOS", "iOS", "Mac"}

	// A fake UA is best not identified as Apple running Android or GNU/Linux.
	if info.brand == "Apple" && !contains(appleOsNames, osName) {
		info.deviceType = device.TypeUnknown
		info.brand = ""
		info.model = ""
	}

	// Assume all devices running iOS / Mac OS are from Apple.
	if info.brand == "" && contains(appleOsNames, osName) {
		info.brand = "Apple"
	}

	// All devices containing a VR fragment are assumed to be wearables.
	if info.deviceType == device.TypeUnknown && matchUA(ua, `Android( [.0-9]+)?; Mobile VR;| VR `) {
		info.deviceType = device.TypeWearable
	}

	// Chrome on Android: 'Mobile' keyword means smartphone, otherwise tablet.
	if info.deviceType == device.TypeUnknown && osFamily == "Android" && matchUA(ua, `Chrome/[.0-9]*`) {
		if matchUA(ua, `(?:Mobile|eliboM)`) {
			info.deviceType = device.TypeSmartphone
		} else {
			info.deviceType = device.TypeTablet
		}
	}

	// UAs with 'Pad/APad' are tablets, not smartphones.
	if info.deviceType == device.TypeSmartphone && matchUA(ua, `Pad/APad`) {
		info.deviceType = device.TypeTablet
	}

	// 'Android; Tablet;' or 'Opera Tablet' fragments mean tablet.
	if info.deviceType == device.TypeUnknown &&
		(matchUA(ua, `Android( [.0-9]+)?; Tablet;|Tablet(?! PC)|.*\-tablet$`) || matchUA(ua, `Opera Tablet`)) {
		info.deviceType = device.TypeTablet
	}

	// 'Android; Mobile;' fragment means smartphone.
	if info.deviceType == device.TypeUnknown && matchUA(ua, `Android( [.0-9]+)?; Mobile;|.*\-mobile$`) {
		info.deviceType = device.TypeSmartphone
	}

	// Android < 2 was smartphone-only, 3.x tablet-only; 2.x and 4.x+ unknown.
	if info.deviceType == device.TypeUnknown && osName == "Android" && osVersion != Unknown && osVersion != "" {
		if versionCompare(osVersion, "2.0") < 0 {
			info.deviceType = device.TypeSmartphone
		} else if versionCompare(osVersion, "3.0") >= 0 && versionCompare(osVersion, "4.0") < 0 {
			info.deviceType = device.TypeTablet
		}
	}

	// Feature phones running Android are more likely smartphones.
	if info.deviceType == device.TypeFeaturePhone && osFamily == "Android" {
		info.deviceType = device.TypeSmartphone
	}

	// Unknown devices running Java ME are more likely feature phones.
	if osName == "Java ME" && info.deviceType == device.TypeUnknown {
		info.deviceType = device.TypeFeaturePhone
	}

	// All devices running KaiOS are more likely feature phones.
	if osName == "KaiOS" {
		info.deviceType = device.TypeFeaturePhone
	}

	// Windows 8+ touch devices are assumed to be tablets.
	if info.deviceType == device.TypeUnknown &&
		(osName == "Windows RT" || (osName == "Windows" && osVersion != Unknown && versionCompare(osVersion, "8") >= 0)) &&
		info.IsTouchEnabled() {
		info.deviceType = device.TypeTablet
	}

	// Puffin desktop / smartphone / tablet markers.
	if info.deviceType == device.TypeUnknown && matchUA(ua, `Puffin/(?:\d+[.\d]+)[LMW]D`) {
		info.deviceType = device.TypeDesktop
	}

	if info.deviceType == device.TypeUnknown && matchUA(ua, `Puffin/(?:\d+[.\d]+)[AIFLW]P`) {
		info.deviceType = device.TypeSmartphone
	}

	if info.deviceType == device.TypeUnknown && matchUA(ua, `Puffin/(?:\d+[.\d]+)[AILW]T`) {
		info.deviceType = device.TypeTablet
	}

	// Opera TV Store / OMI devices are TVs.
	if matchUA(ua, `Opera TV Store| OMI/`) {
		info.deviceType = device.TypeTV
	}

	// Coolita OS devices are coocaa TVs.
	if osName == "Coolita OS" {
		info.deviceType = device.TypeTV
		info.brand = "coocaa"
	}

	// 'Andr0id', 'Android TV', 'BRAVIA', trailing ' TV' etc. mean TV.
	if info.deviceType != device.TypeTV && info.deviceType != device.TypePeripheral &&
		matchUA(ua, `Andr0id|(?:Android(?: UHD)?|Google) TV|\(lite\) TV|BRAVIA|Firebolt| TV$`) {
		info.deviceType = device.TypeTV
	}

	// Tizen TV / SmartTV markers.
	if info.deviceType == device.TypeUnknown && matchUA(ua, `SmartTV|Tizen.+ TV .+$`) {
		info.deviceType = device.TypeTV
	}

	// Clients only ever seen on TVs.
	tvClients := []string{
		"Kylo", "Espial TV Browser", "LUJO TV Browser", "LogicUI TV Browser", "Open TV Browser", "Seraphic Sraf",
		"Opera Devices", "Crow Browser", "Vewd Browser", "TiviMate", "Quick Search TV", "QJY TV Browser", "TV Bro",
	}
	if contains(tvClients, clientName) {
		info.deviceType = device.TypeTV
	}

	// '(TV;' fragment means TV.
	if info.deviceType == device.TypeUnknown && matchUA(ua, `\(TV;`) {
		info.deviceType = device.TypeTV
	}

	// Explicit 'Desktop x64;'-style fragment forces desktop.
	if info.deviceType != device.TypeDesktop && strings.Contains(ua, "Desktop") &&
		matchUA(ua, `Desktop(?: (x(?:32|64)|WOW64))?;`) {
		info.deviceType = device.TypeDesktop
	}

	// Anything else running a desktop OS is a desktop.
	if info.deviceType == device.TypeUnknown && info.IsDesktop() {
		info.deviceType = device.TypeDesktop
	}

	return nil
}

var ddRegexCache sync.Map // string -> *regexp2.Regexp

// matchUA mirrors DeviceDetector::matchUserAgent(). Its anchor,
// `(?:^|[^A-Z_-])`, is deliberately looser than AbstractParser's
// (`[^A-Z0-9_-]` plus underscore/vendor guards): digits may precede a match,
// so e.g. the ' TV$' heuristic fires on "…Safari/537.36 TV". Keep the two
// anchors distinct — parsers use parser.MatchUserAgent, heuristics use this.
func matchUA(ua, pattern string) bool {
	cached, ok := ddRegexCache.Load(pattern)
	if !ok {
		re, err := regexp2.Compile(`(?:^|[^A-Z_-])(?:`+pattern+`)`, regexp2.IgnoreCase)
		if err != nil {
			return false
		}

		ddRegexCache.Store(pattern, re)
		cached = re
	}

	m, err := cached.(*regexp2.Regexp).FindStringMatch(ua)

	return err == nil && m != nil
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}

	return false
}

// versionCompare replicates PHP version_compare() for the dotted-numeric
// versions this library feeds it: parts are compared numerically and, on a
// common prefix, the version with more parts is considered greater
// ("2" < "2.0"). Returns -1, 0 or 1.
func versionCompare(a, b string) int {
	pa := strings.FieldsFunc(a, func(r rune) bool { return r == '.' })
	pb := strings.FieldsFunc(b, func(r rune) bool { return r == '.' })

	for i := 0; i < len(pa) && i < len(pb); i++ {
		na, _ := strconv.Atoi(pa[i])
		nb, _ := strconv.Atoi(pb[i])

		if na != nb {
			if na < nb {
				return -1
			}

			return 1
		}
	}

	switch {
	case len(pa) < len(pb):
		return -1
	case len(pa) > len(pb):
		return 1
	default:
		return 0
	}
}
