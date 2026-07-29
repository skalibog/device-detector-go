package devicedetector

import (
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dlclark/regexp2"

	"github.com/skalibog/device-detector-go/internal/parser"
	"github.com/skalibog/device-detector-go/internal/parser/client"
	"github.com/skalibog/device-detector-go/internal/parser/device"
)

// unknown is the value PHP DeviceDetector reports for unknown attributes.
const unknown = "UNK"

// DefaultMaxUARawLength bounds attacker-controlled input: user agents longer
// than this are truncated before parsing. The longest real user agent in the
// upstream corpus is ~400 bytes, so this never affects genuine traffic while
// capping the cost of oversized junk. Override with WithMaxUARawLength.
const DefaultMaxUARawLength = 2048

// DeviceDetector parses user agent strings. It is immutable after
// construction and safe for concurrent use.
type DeviceDetector struct {
	bot           *parser.Bot
	os            *parser.OS
	vendor        *parser.VendorFragment
	clientParsers []client.Parser
	deviceParsers []device.Parser

	skipBotDetection bool
	truncation       VersionTruncation
	maxUALen         int
	lazyCompile      bool
}

// Option configures a DeviceDetector.
type Option func(*DeviceDetector)

// WithVersionTruncation sets how detailed reported versions are
// (VersionTruncation* constants). Default is minor precision, matching the PHP
// library. An unrecognised value is ignored (the default is kept), mirroring
// AbstractParser::setVersionTruncation.
func WithVersionTruncation(t VersionTruncation) Option {
	return func(d *DeviceDetector) {
		switch t {
		case VersionTruncationMajor, VersionTruncationMinor, VersionTruncationPatch,
			VersionTruncationBuild, VersionTruncationNone:
			d.truncation = t
		}
	}
}

// WithSkipBotDetection disables bot detection entirely, mirroring
// DeviceDetector::skipBotDetection().
func WithSkipBotDetection() Option {
	return func(d *DeviceDetector) { d.skipBotDetection = true }
}

// WithMaxUARawLength truncates user agents longer than n bytes before parsing,
// bounding the cost of oversized attacker input. n <= 0 disables truncation
// (exact upstream parity, which imposes no limit). Defaults to
// DefaultMaxUARawLength.
func WithMaxUARawLength(n int) Option {
	return func(d *DeviceDetector) { d.maxUALen = n }
}

// WithLazyCompile defers regex compilation to first use instead of compiling
// the whole database at construction. New becomes faster and lighter, at the
// cost of losing the fail-fast guarantee: a syntactically broken pattern in an
// external database then surfaces on the first user agent that reaches it,
// rather than from New. The embedded database is always valid, so this is only
// relevant with NewFromDir/NewFromFS.
func WithLazyCompile() Option {
	return func(d *DeviceDetector) { d.lazyCompile = true }
}

// WithMatchTimeout bounds how long any single regex match may run before it is
// abandoned, protecting against catastrophic backtracking on crafted input;
// d <= 0 disables it. See DefaultMatchTimeout.
//
// The regex cache is process-wide, so this setting is process-global rather
// than per-detector: a pattern keeps the timeout in effect when it was first
// compiled. Set it consistently, before constructing your first detector.
func WithMatchTimeout(d time.Duration) Option {
	return func(*DeviceDetector) { parser.SetMatchTimeout(d) }
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
	d := &DeviceDetector{truncation: VersionTruncationMinor, maxUALen: DefaultMaxUARawLength}

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

	d.os.SetVersionTruncation(int(d.truncation))

	if d.vendor, err = parser.NewVendorFragment(fsys); err != nil {
		return nil, err
	}

	if d.clientParsers, err = client.All(fsys); err != nil {
		return nil, err
	}

	for _, p := range d.clientParsers {
		p.SetVersionTruncation(int(d.truncation))
	}

	if d.deviceParsers, err = device.All(fsys); err != nil {
		return nil, err
	}

	// Fail-fast: compile the whole database up front so a broken pattern in an
	// external database surfaces here rather than on some later Parse. Cheap
	// (~130 ms for the embedded database). Opt out with WithLazyCompile.
	if !d.lazyCompile {
		for _, p := range d.clientParsers {
			if err := p.Warm(); err != nil {
				return nil, err
			}
		}

		for _, p := range d.deviceParsers {
			if err := p.Warm(); err != nil {
				return nil, err
			}
		}
	}

	return d, nil
}

// Info holds the outcome of parsing a single user agent.
type Info struct {
	UserAgent string

	bot        *Bot
	os         *OS
	client     *Client
	deviceType DeviceType
	brand      string
	model      string
}

// IsBot reports whether the UA was identified as a bot.
func (i *Info) IsBot() bool { return i.bot != nil }

// Bot returns bot details, or nil for non-bot traffic.
func (i *Info) Bot() *Bot { return i.bot }

// OS returns operating system details, or nil when undetected.
func (i *Info) OS() *OS { return i.os }

// Client returns client (browser/app/...) details, or nil when undetected.
func (i *Info) Client() *Client { return i.client }

// DeviceType returns the detected device type (DeviceType* constants),
// or DeviceTypeUnknown.
func (i *Info) DeviceType() DeviceType { return i.deviceType }

// DeviceName returns the canonical device type name ("smartphone", ...),
// or "" when unknown.
func (i *Info) DeviceName() string { return i.deviceType.String() }

// Brand returns the device brand name, or "".
func (i *Info) Brand() string { return i.brand }

// Model returns the device model, or "".
func (i *Info) Model() string { return i.model }

func (i *Info) osAttr(get func(*OS) string) string {
	if i.os == nil {
		return unknown
	}

	if v := get(i.os); v != "" {
		return v
	}

	return unknown
}

// IsTouchEnabled mirrors DeviceDetector::isTouchEnabled().
func (i *Info) IsTouchEnabled() bool { return matchUA(i.UserAgent, `Touch`) }

func (i *Info) usesMobileBrowser() bool {
	return i.client != nil && i.client.Type == ClientBrowser &&
		client.IsMobileOnlyBrowser(i.client.Name)
}

// IsDesktop mirrors DeviceDetector::isDesktop(): unknown-type devices running
// a desktop OS with a non-mobile-only browser.
func (i *Info) IsDesktop() bool {
	osName := i.osAttr(func(o *OS) string { return o.Name })
	if osName == unknown {
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
	case DeviceTypeFeaturePhone, DeviceTypeSmartphone, DeviceTypeTablet,
		DeviceTypePhablet, DeviceTypeCamera, DeviceTypePortableMediaPlayer:
		return true
	case DeviceTypeTV, DeviceTypeSmartDisplay, DeviceTypeConsole:
		return false
	}

	if i.usesMobileBrowser() {
		return true
	}

	if i.os == nil || i.os.Name == "" || i.os.Name == unknown {
		return false
	}

	return !i.IsBot() && !i.IsDesktop()
}

var hasLetterRe = regexp.MustCompile(`[a-zA-Z]`)

// IsBot reports whether ua belongs to a known bot. It evaluates only the
// combined bot regex — a cheap short-circuit, much faster than a full Parse —
// for callers that only need bot filtering.
func (d *DeviceDetector) IsBot(ua string) (bool, error) {
	if d.maxUALen > 0 && len(ua) > d.maxUALen {
		ua = ua[:d.maxUALen]
	}

	if ua == "" || !hasLetterRe.MatchString(ua) {
		return false, nil
	}

	return d.bot.IsBot(ua)
}

// Parse runs the full detection pipeline on ua, mirroring
// DeviceDetector::parse(). User agents longer than the configured maximum are
// truncated first (see WithMaxUARawLength).
func (d *DeviceDetector) Parse(ua string) (*Info, error) {
	if d.maxUALen > 0 && len(ua) > d.maxUALen {
		ua = ua[:d.maxUALen]
	}

	info := &Info{UserAgent: ua, deviceType: DeviceTypeUnknown}

	if ua == "" || !hasLetterRe.MatchString(ua) {
		return info, nil
	}

	// On a stage error (e.g. a match timeout on adversarial input, or a bad
	// pattern in an external database) Parse returns the partial Info built so
	// far alongside the error rather than discarding it, so callers can treat
	// the result as best-effort. Info is never nil.
	if !d.skipBotDetection {
		bot, err := d.bot.Parse(ua)
		if err != nil {
			return info, err
		}

		if bot != nil {
			info.bot = botFrom(bot)

			return info, nil
		}
	}

	osResult, err := d.os.Parse(ua)
	if err != nil {
		return info, err
	}

	info.os = osFrom(osResult)

	for _, p := range d.clientParsers {
		res, err := p.Parse(ua)
		if err != nil {
			return info, err
		}

		if res != nil {
			info.client = clientFrom(res)

			break
		}
	}

	if err := d.parseDevice(info); err != nil {
		return info, err
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
			info.deviceType = DeviceType(res.Type)
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

	osName := info.osAttr(func(o *OS) string { return o.Name })
	osFamily := info.osAttr(func(o *OS) string { return o.Family })
	osVersion := info.osAttr(func(o *OS) string { return o.Version })
	clientName := ""

	if info.client != nil {
		clientName = info.client.Name
	}

	appleOsNames := []string{"iPadOS", "tvOS", "watchOS", "iOS", "Mac"}

	// A fake UA is best not identified as Apple running Android or GNU/Linux.
	if info.brand == "Apple" && !contains(appleOsNames, osName) {
		info.deviceType = DeviceTypeUnknown
		info.brand = ""
		info.model = ""
	}

	// Assume all devices running iOS / Mac OS are from Apple.
	if info.brand == "" && contains(appleOsNames, osName) {
		info.brand = "Apple"
	}

	// All devices containing a VR fragment are assumed to be wearables.
	if info.deviceType == DeviceTypeUnknown && matchUA(ua, `Android( [.0-9]+)?; Mobile VR;| VR `) {
		info.deviceType = DeviceTypeWearable
	}

	// Chrome on Android: 'Mobile' keyword means smartphone, otherwise tablet.
	if info.deviceType == DeviceTypeUnknown && osFamily == "Android" && matchUA(ua, `Chrome/[.0-9]*`) {
		if matchUA(ua, `(?:Mobile|eliboM)`) {
			info.deviceType = DeviceTypeSmartphone
		} else {
			info.deviceType = DeviceTypeTablet
		}
	}

	// UAs with 'Pad/APad' are tablets, not smartphones.
	if info.deviceType == DeviceTypeSmartphone && matchUA(ua, `Pad/APad`) {
		info.deviceType = DeviceTypeTablet
	}

	// 'Android; Tablet;' or 'Opera Tablet' fragments mean tablet.
	if info.deviceType == DeviceTypeUnknown &&
		(matchUA(ua, `Android( [.0-9]+)?; Tablet;|Tablet(?! PC)|.*\-tablet$`) || matchUA(ua, `Opera Tablet`)) {
		info.deviceType = DeviceTypeTablet
	}

	// 'Android; Mobile;' fragment means smartphone.
	if info.deviceType == DeviceTypeUnknown && matchUA(ua, `Android( [.0-9]+)?; Mobile;|.*\-mobile$`) {
		info.deviceType = DeviceTypeSmartphone
	}

	// Android < 2 was smartphone-only, 3.x tablet-only; 2.x and 4.x+ unknown.
	if info.deviceType == DeviceTypeUnknown && osName == "Android" && osVersion != unknown && osVersion != "" {
		if versionCompare(osVersion, "2.0") < 0 {
			info.deviceType = DeviceTypeSmartphone
		} else if versionCompare(osVersion, "3.0") >= 0 && versionCompare(osVersion, "4.0") < 0 {
			info.deviceType = DeviceTypeTablet
		}
	}

	// Feature phones running Android are more likely smartphones.
	if info.deviceType == DeviceTypeFeaturePhone && osFamily == "Android" {
		info.deviceType = DeviceTypeSmartphone
	}

	// Unknown devices running Java ME are more likely feature phones.
	if osName == "Java ME" && info.deviceType == DeviceTypeUnknown {
		info.deviceType = DeviceTypeFeaturePhone
	}

	// All devices running KaiOS are more likely feature phones.
	if osName == "KaiOS" {
		info.deviceType = DeviceTypeFeaturePhone
	}

	// Windows 8+ touch devices are assumed to be tablets.
	if info.deviceType == DeviceTypeUnknown &&
		(osName == "Windows RT" || (osName == "Windows" && osVersion != unknown && versionCompare(osVersion, "8") >= 0)) &&
		info.IsTouchEnabled() {
		info.deviceType = DeviceTypeTablet
	}

	// Puffin desktop / smartphone / tablet markers.
	if info.deviceType == DeviceTypeUnknown && matchUA(ua, `Puffin/(?:\d+[.\d]+)[LMW]D`) {
		info.deviceType = DeviceTypeDesktop
	}

	if info.deviceType == DeviceTypeUnknown && matchUA(ua, `Puffin/(?:\d+[.\d]+)[AIFLW]P`) {
		info.deviceType = DeviceTypeSmartphone
	}

	if info.deviceType == DeviceTypeUnknown && matchUA(ua, `Puffin/(?:\d+[.\d]+)[AILW]T`) {
		info.deviceType = DeviceTypeTablet
	}

	// Opera TV Store / OMI devices are TVs.
	if matchUA(ua, `Opera TV Store| OMI/`) {
		info.deviceType = DeviceTypeTV
	}

	// Coolita OS devices are coocaa TVs.
	if osName == "Coolita OS" {
		info.deviceType = DeviceTypeTV
		info.brand = "coocaa"
	}

	// 'Andr0id', 'Android TV', 'BRAVIA', trailing ' TV' etc. mean TV.
	if info.deviceType != DeviceTypeTV && info.deviceType != DeviceTypePeripheral &&
		matchUA(ua, `Andr0id|(?:Android(?: UHD)?|Google) TV|\(lite\) TV|BRAVIA|Firebolt| TV$`) {
		info.deviceType = DeviceTypeTV
	}

	// Tizen TV / SmartTV markers.
	if info.deviceType == DeviceTypeUnknown && matchUA(ua, `SmartTV|Tizen.+ TV .+$`) {
		info.deviceType = DeviceTypeTV
	}

	// Clients only ever seen on TVs.
	tvClients := []string{
		"Kylo", "Espial TV Browser", "LUJO TV Browser", "LogicUI TV Browser", "Open TV Browser", "Seraphic Sraf",
		"Opera Devices", "Crow Browser", "Vewd Browser", "TiviMate", "Quick Search TV", "QJY TV Browser", "TV Bro",
	}
	if contains(tvClients, clientName) {
		info.deviceType = DeviceTypeTV
	}

	// '(TV;' fragment means TV.
	if info.deviceType == DeviceTypeUnknown && matchUA(ua, `\(TV;`) {
		info.deviceType = DeviceTypeTV
	}

	// Explicit 'Desktop x64;'-style fragment forces desktop.
	if info.deviceType != DeviceTypeDesktop && strings.Contains(ua, "Desktop") &&
		matchUA(ua, `Desktop(?: (x(?:32|64)|WOW64))?;`) {
		info.deviceType = DeviceTypeDesktop
	}

	// Anything else running a desktop OS is a desktop.
	if info.deviceType == DeviceTypeUnknown && info.IsDesktop() {
		info.deviceType = DeviceTypeDesktop
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

		parser.StampTimeout(re)
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
