package handlers

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"GoBot/core"
	"GoBot/core/dispatch"
)

type distantWorlds struct {
	dispatch.NoOpMessageHandler
}

type Waypoint struct {
	Start string
	End string
	RouteMap string
	Poi []string
}

const (
	WaypointCmd = "wp"
	ReloadCmd = "reloadwp"
)

var waypoints []Waypoint

func (*distantWorlds) CommandGroup() string {
	return "Distant Worlds Commands"
}

func init() {
	dwehandler := distantWorlds{}
	dispatch.Register(&dwehandler,
		[]dispatch.MessageCommand{
			{WaypointCmd, "Get information about waypoints. Syntax: *wp <waypoint #>* or *wp#*." },
			{ReloadCmd, ""},
		},
		[]dispatch.MessageCommand{
			{WaypointCmd, ""},
		},
		false)
}

func (*distantWorlds) SettingsLoaded() {
	core.LogDebug("Loading DWE2 waypoints...")
	loadWaypoints()
}

func (*distantWorlds) HandlePrefix(prefix, suffix string, m *dispatch.Message) bool {
	switch prefix {
	case WaypointCmd:
		m.Args = []string {suffix}
		handleWaypoint(m)
	default:
		return false
	}
	return true
}

func (c *distantWorlds) HandleCommand(m *dispatch.Message) bool {
	switch m.Command {
	case WaypointCmd:
		handleWaypoint(m)
	case ReloadCmd:
		reloadWaypoints(m);
	default:
		return false
	}
	return true
}

func loadWaypoints() bool {
	// Open our jsonFile
	jsonFile, err := os.Open(core.Settings.ResourceDirectory() + "/dwe2-waypoints.json")
	if err != nil {
		core.LogError("Failed to load waypoints file: ", err)
		return false
	}
	defer jsonFile.Close()
	decoder := json.NewDecoder(jsonFile)

	err = decoder.Decode(&waypoints)
	if err != nil {
		core.LogError("Failed to parse DWE waypoints: ", err)
		return false
	}
	core.LogDebug(waypoints)
	return true
}

func reloadWaypoints(m *dispatch.Message) {
	waypoints = nil
	if !loadWaypoints() {
		m.ReplyToSender("Failed to reload waypoints - check log file.")
	}
	m.ReplyToSender("Reloaded waypoints. %d waypoints found", len(waypoints))
}


func handleWaypoint(m *dispatch.Message) {
	var wp int
	switch len(m.Args) {
	case 0:
		m.ReplyToChannel("Missing waypoint.")
		return
	case 1:
		var err error
		wp, err = strconv.Atoi(m.Args[0])
		if err != nil {
			m.ReplyToChannel("Invalid waypoint [%s], expected a number.", m.Args[0])
			return
		}
		if wp <= 0 || wp > len(waypoints) {
			m.ReplyToChannel("Waypoint %d is invalid, or not yet announced (there are currently %d known waypoints, with 15 expected)", wp, len(waypoints))
			return
		}
		break
	default:
		m.ReplyToChannel("Too many arguments, expected waypoint.")
		return
	}
	waypoint := waypoints[wp - 1]
	if !m.IsPM {
		m.ReplyToChannel("%s, sent you a PM with the waypoint information.", m.Author.Username)
	}
	m.ReplyToSender("Waypoint %d: %s\n\nPlaces to visit on the way to the next waypoint:\n\n * %s\n\nArrive at: %s\n\nRoute Map: %s\n\nFor details about base camps and such, see: https://tinyurl.com/yah9wgkj\n",
		wp, waypoint.Start, strings.Join(waypoint.Poi, "\n * "), waypoint.End, waypoint.RouteMap)

}
