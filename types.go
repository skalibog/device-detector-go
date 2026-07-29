package devicedetector

import (
	"github.com/skalibog/device-detector-go/internal/parser"
	"github.com/skalibog/device-detector-go/internal/parser/client"
	"github.com/skalibog/device-detector-go/internal/parser/device"
)

// DeviceType is the class of device detected from a user agent. Its numeric
// values match matomo/device-detector's DEVICE_TYPE_* constants, so raw ids can
// be stored and compared across implementations.
type DeviceType int

// Device type values, mirroring matomo/device-detector.
const (
	DeviceTypeDesktop             DeviceType = 0
	DeviceTypeSmartphone          DeviceType = 1
	DeviceTypeTablet              DeviceType = 2
	DeviceTypeFeaturePhone        DeviceType = 3
	DeviceTypeConsole             DeviceType = 4
	DeviceTypeTV                  DeviceType = 5
	DeviceTypeCarBrowser          DeviceType = 6
	DeviceTypeSmartDisplay        DeviceType = 7
	DeviceTypeCamera              DeviceType = 8
	DeviceTypePortableMediaPlayer DeviceType = 9
	DeviceTypePhablet             DeviceType = 10
	DeviceTypeSmartSpeaker        DeviceType = 11
	DeviceTypeWearable            DeviceType = 12
	DeviceTypePeripheral          DeviceType = 13
	DeviceTypeUnknown             DeviceType = -1
)

// String returns the canonical device type name ("smartphone", "tablet", ...),
// or "" for DeviceTypeUnknown.
func (t DeviceType) String() string {
	if t == DeviceTypeUnknown {
		return ""
	}

	return device.TypeName(int(t))
}

// DeviceTypeFromName maps a canonical name back to its DeviceType, returning
// DeviceTypeUnknown for an unrecognised name.
func DeviceTypeFromName(name string) DeviceType {
	return DeviceType(device.TypeFromName(name))
}

// VersionTruncation controls how many components a reported version keeps.
type VersionTruncation int

// Version truncation levels. Minor is the default, matching the PHP library.
const (
	VersionTruncationMajor VersionTruncation = VersionTruncation(parser.VersionTruncationMajor)
	VersionTruncationMinor VersionTruncation = VersionTruncation(parser.VersionTruncationMinor)
	VersionTruncationPatch VersionTruncation = VersionTruncation(parser.VersionTruncationPatch)
	VersionTruncationBuild VersionTruncation = VersionTruncation(parser.VersionTruncationBuild)
	VersionTruncationNone  VersionTruncation = VersionTruncation(parser.VersionTruncationNone)
)

// ClientType is the category of a detected client.
type ClientType string

// Client type values, matching the strings used by the underlying database.
const (
	ClientBrowser     ClientType = "browser"
	ClientFeedReader  ClientType = "feed reader"
	ClientLibrary     ClientType = "library"
	ClientMediaPlayer ClientType = "mediaplayer"
	ClientMobileApp   ClientType = "mobile app"
	ClientPIM         ClientType = "pim"
)

// Bot describes a detected bot.
type Bot struct {
	Name     string
	Category string
	URL      string
	Producer BotProducer
}

// BotProducer identifies the organisation behind a bot.
type BotProducer struct {
	Name string
	URL  string
}

// OS describes a detected operating system. Fields are "" when unknown.
type OS struct {
	Name      string
	ShortName string
	Version   string
	Platform  string
	Family    string
}

// Client describes a detected client (browser, app, media player, ...).
// Engine, EngineVersion and Family are populated for browsers only.
type Client struct {
	Type          ClientType
	Name          string
	Version       string
	Engine        string
	EngineVersion string
	Family        string
}

func botFrom(r *parser.BotResult) *Bot {
	if r == nil {
		return nil
	}

	return &Bot{
		Name:     r.Name,
		Category: r.Category,
		URL:      r.URL,
		Producer: BotProducer{Name: r.Producer.Name, URL: r.Producer.URL},
	}
}

func osFrom(r *parser.OSResult) *OS {
	if r == nil {
		return nil
	}

	short := r.ShortName
	if short == unknown {
		short = ""
	}

	return &OS{
		Name:      r.Name,
		ShortName: short,
		Version:   r.Version,
		Platform:  r.Platform,
		Family:    r.Family,
	}
}

func clientFrom(r *client.Result) *Client {
	if r == nil {
		return nil
	}

	return &Client{
		Type:          ClientType(r.Type),
		Name:          r.Name,
		Version:       r.Version,
		Engine:        r.Engine,
		EngineVersion: r.EngineVersion,
		Family:        r.Family,
	}
}
