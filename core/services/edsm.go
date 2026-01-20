package services

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	"GoBot/core"
)

// SystemCoords represents coordinates for a star system
type SystemCoords struct {
	X, Y, Z float64
}

// SystemInfo contains system name and coordinates from EDSM
type SystemInfo struct {
	Name   string        `json:"name"`
	Coords *SystemCoords `json:"coords"`
}

var (
	// Cache for system coordinates
	systemCoordsCache   = make(map[string]*SystemCoords)
	systemCoordsCacheMu sync.RWMutex
	systemCacheExpiry   = make(map[string]time.Time)
	cacheDuration       = 24 * time.Hour // Cache coords for 24 hours

	edsmClient = &http.Client{Timeout: 10 * time.Second}
)

// GetSystemCoords fetches coordinates for a system from EDSM (with caching)
func GetSystemCoords(systemName string) (*SystemCoords, error) {
	if systemName == "" {
		return nil, nil
	}

	// Check cache first
	systemCoordsCacheMu.RLock()
	if coords, ok := systemCoordsCache[systemName]; ok {
		if time.Now().Before(systemCacheExpiry[systemName]) {
			systemCoordsCacheMu.RUnlock()
			return coords, nil
		}
	}
	systemCoordsCacheMu.RUnlock()

	// Fetch from EDSM
	u, err := core.MakeURL("https://www.edsm.net/api-v1/system", []core.URLParams{
		{"systemName", systemName},
		{"coords", "1"},
	})
	if err != nil {
		return nil, err
	}

	resp, err := edsmClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// EDSM returns [] for unknown systems
	if string(body) == "[]" || len(body) < 3 {
		// System not found - cache nil result
		systemCoordsCacheMu.Lock()
		systemCoordsCache[systemName] = nil
		systemCacheExpiry[systemName] = time.Now().Add(cacheDuration)
		systemCoordsCacheMu.Unlock()
		return nil, nil
	}

	var system SystemInfo
	if err := json.Unmarshal(body, &system); err != nil {
		return nil, err
	}

	if system.Coords == nil {
		// System exists but not trilaterated - cache nil result too
		systemCoordsCacheMu.Lock()
		systemCoordsCache[systemName] = nil
		systemCacheExpiry[systemName] = time.Now().Add(cacheDuration)
		systemCoordsCacheMu.Unlock()
		return nil, nil
	}

	// Cache the result
	systemCoordsCacheMu.Lock()
	systemCoordsCache[systemName] = system.Coords
	systemCacheExpiry[systemName] = time.Now().Add(cacheDuration)
	systemCoordsCacheMu.Unlock()

	return system.Coords, nil
}

// CalculateDistance calculates the distance in light years between two coordinate sets
func CalculateDistance(c1, c2 *SystemCoords) float64 {
	if c1 == nil || c2 == nil {
		return -1
	}

	sq := func(a, b float64) float64 {
		return math.Pow(a-b, 2)
	}

	return math.Sqrt(sq(c1.X, c2.X) + sq(c1.Y, c2.Y) + sq(c1.Z, c2.Z))
}

// GetDistanceBetweenSystems calculates distance between two systems by name
func GetDistanceBetweenSystems(system1, system2 string) (float64, error) {
	coords1, err := GetSystemCoords(system1)
	if err != nil {
		return -1, err
	}

	coords2, err := GetSystemCoords(system2)
	if err != nil {
		return -1, err
	}

	return CalculateDistance(coords1, coords2), nil
}
