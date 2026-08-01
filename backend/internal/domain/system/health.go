package system

type RuntimeTargets struct {
	Go    string
	Bun   string
	React string
}

type QueueWorkerHealth struct {
	Enabled             bool
	Ready               bool
	IntervalMS          int
	LastAttemptAt       int64
	LastSuccessAt       int64
	LagSeconds          int64
	ConsecutiveFailures int
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
	MasterDBReady     bool
	UniverseDBReady   bool
	ModRuntimeReady   bool
	QueueWorker       QueueWorkerHealth
}
