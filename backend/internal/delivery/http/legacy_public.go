package httpdelivery

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
