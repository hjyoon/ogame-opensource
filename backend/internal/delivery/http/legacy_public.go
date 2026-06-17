package httpdelivery

// These paths are only compatibility entry points for old bookmarks and tests.
// New React routes should use natural application paths, not PHP-style names.
var legacyPublicHTMLPaths = map[string]struct{}{
	"/about.php":       {},
	"/home.php":        {},
	"/impressum.php":   {},
	"/index.php":       {},
	"/install.php":     {},
	"/register.php":    {},
	"/regeln.php":      {},
	"/screenshots.php": {},
	"/story.php":       {},
	"/unis.php":        {},
}

func isLegacyPublicHTMLPath(cleanPath string) bool {
	_, ok := legacyPublicHTMLPaths[cleanPath]
	return ok
}
