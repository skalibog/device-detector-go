// Package parser contains the core detection machinery shared by all
// stages of devicedetector: anchored regex matching (a faithful port of the
// upstream PHP matching semantics on top of the regexp2 engine), YAML
// database loading with order-preserving maps, version building and
// truncation, plus the bot, operating-system and vendor-fragment parsers.
//
// Most applications should use the root devicedetector package instead of
// this one.
package parser
