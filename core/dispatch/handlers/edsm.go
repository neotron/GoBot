package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"GoBot/core"
	"GoBot/core/dispatch"
	"github.com/thoas/go-funk"
)

//
// Created by David Hedbor on 2/25/16.
// Copyright (c) 2016 NeoTron. All rights reserved.
//

type edsm struct {
	dispatch.NoOpMessageHandler
}

// {"msgnum":100,"msg":"OK","system":"Phipoea DD-F c26-1311","firstDiscover":false,"date":"2016-02-25 06:44:54"}

type CommanderPositionModel struct {
	Msg           string
	Msgnum        int
	System        string
	FirstDiscover bool
	Date          string
}
type CoordModel struct {
	X, Y, Z float64
}

type SystemModel struct {
	Name   string
	Coords *CoordModel
}

func init() {
	dispatch.Register(&edsm{},
		[]dispatch.MessageCommand{
			{"loc", "Try to get a commanders location from EDSM. Syntax: loc <commander name>"},
			{"dist", "Calculate distance between two systems. Syntax: dist <system> -> <system> (i.e: `dist Sol -> Sagittarius A*`)"},
		}, nil, false)
}

func (s *edsm) handleCommand(m *dispatch.Message) bool {
	switch m.Command {
	case "loc":
		go handleLocationLookup(strings.Join(m.Args, " "), m)
	case "dist":
		systems := funk.Map(strings.Split(strings.Join(m.Args, " "), "->"), func(arg string) string {
			return strings.Trim(arg, " \t\n")
		}).([]string)
		if len(systems) == 1 {
			systems = append(systems, "Sol")
		}
		handleDistance(systems, m)
	default:
		return false
	}
	return true
}

func fetchCommanderLocation(commander string, m *dispatch.Message) *CommanderPositionModel {
	u, err := core.MakeURL("https://www.edsm.net/api-logs-v1/get-position/", []core.URLParams{
		{"commanderName", commander},
	})
	res, err := http.Get(u.String())
	if err != nil {
		core.LogError("Failed to query ESDM for commander location: ", err)
		m.ReplyToChannel("Failed to complete request.")
		return nil
	}

	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	var cmdr *CommanderPositionModel
	err = decoder.Decode(&cmdr)
	if err != nil {
		core.LogError("Failed to decode ESDM query response: ", err)
		m.ReplyToChannel("Failed to parse ESDM query response.")
		return nil
	}
	return cmdr
}
func handleLocationLookup(commander string, m *dispatch.Message) {

	c := fetchCommanderLocation(commander, m)

	if len(c.System) > 0 {
		var output = fmt.Sprint(commander, " was last seen in ", c.System)
		if len(c.Date) > 0 {
			output = fmt.Sprint(output, " at ", c.Date)
		}
		m.ReplyToChannel(output)
	} else {
		switch c.Msgnum {
		case 100:
			m.ReplyToChannel("I have no idea where %s is - perhaps they aren't sharing their position?", commander)
		case 203:
			m.ReplyToChannel("There's no known commander by the name %s.", commander)
		default:
			m.ReplyToChannel("Some error happened.")
		}
	}
}

func (*edsm) CommandGroup() string {
	return "EDSM Queries"
}

func handleDistance(s []string, m *dispatch.Message) {
	var aliases = map[string]string{
		"jaques":         "Colonia",
		"jaques station": "Colonia",
	}
	if len(s) != 2 {
		m.ReplyToChannel("Invalid syntax. Expected: '%sdist System Name -> System 2 Name`", core.Settings.CommandPrefix())
		return
	}
	var systemCoords []SystemModel
	calcDist := func(model SystemModel) {
		systemCoords = append(systemCoords, model)
		if len(systemCoords) == 2 {
			calculateDistance(systemCoords, m)
		}
	}
	for _, systemName := range s {
		var waypointName string
		if alias := aliases[strings.ToLower(systemName)]; len(alias) > 0 {
			waypointName = fmt.Sprintf("%s `(aka %s)`", systemName, alias)
			systemName = alias
		}
		parts := strings.Split(systemName, " ")

		// Parsed raw coordinates
		switch len(parts) {
		case 3:
			x, errX := strconv.ParseFloat(parts[0], 64)
			y, errY := strconv.ParseFloat(parts[1], 64)
			z, errZ := strconv.ParseFloat(parts[2], 64)
			if errX == nil && errY == nil && errZ == nil {
				sys := SystemModel{
					fmt.Sprintf("`(x: %.1f, y: %.1f, z: %.1f)`", x, y, z),
					&CoordModel{x, y, z},
				}
				calcDist(sys)
				continue
			}
		case 1:

			// TODO: Waypoint handling
			_, err := strconv.ParseInt(parts[0], 10, 8)
			if err == nil {
				//	wps := DistantWorldsWaypoints.database?.waypoints else {
				m.ReplyToChannel("Failed to load waypoint database, sorry.")
				//		return
			}
			/*	if wp < 0 || wp >= wps.count {
					m.ReplyToChannel("Waypoint \(wp) is not valid.")
					return
				}
				if wps[wp].system == "TBA" {
					m.ReplyToChannel("Waypoint \(wp)'s system is not known yet.")
					return
				}
				systemName = wps[wp].system
				waypointName = "Waypoint \(wp) (\(systemName))"
			*/
			//			}
		}

		// Look up system coordinates by name
		system := getSystemCoords(systemName, m)
		if system != nil {
			if len(waypointName) > 0 {
				system.Name = waypointName
			}
			calcDist(*system)
			continue
		}

		// Check if the "system" is a commander
		location := fetchCommanderLocation(systemName, m)

		if location != nil && len(location.System) > 0 {
			sys := getSystemCoords(location.System, m)
			if sys != nil {
				sys.Name = fmt.Sprintf("Cmdr %s `(in %s)`", systemName, sys.Name)
				calcDist(*sys)
			} else {
				reportNotTrilaterated(systemName, m)
			}
		} else {
			reportNotTrilaterated(systemName, m)
		}
	}
}

func getSystemCoords(systemName string, m *dispatch.Message) *SystemModel {
	u, err := core.MakeURL("https://www.edsm.net/api-v1/system", []core.URLParams{
		{"systemName", systemName},
		{"coords", "1"},
	})

	if err != nil {
		m.ReplyToChannel("Failed to form EDSM request.")
		core.LogError("Failed to make URL: ", err)
		return nil
	}

	res, err := http.Get(u.String())
	if err != nil {
		m.ReplyToChannel("Failed to complete request.")
		core.LogError("Get Position api failed with error: ", err)
		return nil
	}
	defer res.Body.Close()
	var system SystemModel
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&system)

	if err != nil || len(system.Name) == 0 {
		return nil
	}
	if system.Coords == nil {
		reportNotTrilaterated(systemName, m)
		return nil
	}
	return &system
}

func reportNotTrilaterated(systemName string, m *dispatch.Message) {
	m.ReplyToChannel("%s has not been trilaterated.", systemName)
}

func calculateDistance(s []SystemModel, m *dispatch.Message) {
	c1 := s[0].Coords
	c2 := s[1].Coords
	if c1 == nil || c2 == nil {
		m.ReplyToChannel("Couldn't get coordinates for both systems.")
		return
	}

	sq2 := func(a, b float64) float64 {
		val := a - b
		return math.Pow(val, 2)
	}

	dist := math.Sqrt(sq2(c1.X, c2.X) + sq2(c1.Y, c2.Y) + sq2(c1.Z, c2.Z))
	m.ReplyToChannel("Distance between %s and %s is %.2f ly", s[0].Name, s[1].Name, dist)
}
