package system

type RuntimeTargets struct {
	Go    string
	Bun   string
	React string
}

type Health struct {
	Status            string
	Service           string
	Environment       string
	Runtime           string
	Targets           RuntimeTargets
	StaticReady       bool
	LegacyAssetsReady bool
	LegacyBaseURL     string
}
