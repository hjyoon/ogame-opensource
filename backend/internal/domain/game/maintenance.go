package game

type Maintenance struct {
	Frozen   bool
	Language string
	BoardURL string
}

func NormalizeMaintenanceLanguage(language string) string {
	switch language {
	case "de", "en", "es", "fr", "it", "ru":
		return language
	default:
		return "en"
	}
}
