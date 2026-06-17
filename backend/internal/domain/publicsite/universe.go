package publicsite

import (
	"errors"
	"strings"
)

type UniverseStatus string

const (
	UniverseOnline  UniverseStatus = "online"
	UniverseOffline UniverseStatus = "offline"
)

type Universe struct {
	Number     int
	Name       string
	BaseURL    string
	Language   string
	Speed      int
	FleetSpeed int
	Status     UniverseStatus
}

func (u Universe) Validate() error {
	if u.Number <= 0 {
		return errors.New("universe number must be positive")
	}
	if strings.TrimSpace(u.Name) == "" {
		return errors.New("universe name is required")
	}
	if strings.TrimSpace(u.BaseURL) == "" {
		return errors.New("universe base URL is required")
	}
	if strings.TrimSpace(u.Language) == "" {
		return errors.New("universe language is required")
	}
	if u.Speed <= 0 || u.FleetSpeed <= 0 {
		return errors.New("universe speeds must be positive")
	}
	if u.Status != UniverseOnline && u.Status != UniverseOffline {
		return errors.New("universe status is invalid")
	}
	return nil
}

func (u Universe) Open() bool {
	return u.Status == UniverseOnline
}
