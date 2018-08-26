//
// Created by David Hedbor on 2/13/16.
// Copyright (c) 2016 NeoTron. All rights reserved.
//
// Some functionality especially for Elite: Dangerous to calculcate gravity, core route distances tc.
// Limited functionality outside of E:D :-)

package handlers

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"GoBot/core/dispatch"
)

type science struct {
	dispatch.NoOpMessageHandler
}

func init() {
	dispatch.Register(&science{},
		[]dispatch.MessageCommand{
			{"bearing", "Calculate bearing and optional distance between two planetary coordiantes. Args: <lat1> <lon1> <lat2> <lon2> [planet radius in km]"},
			{"g", "Calculate gravity for a planet. Arguments: <Earth masses> <radius in km>"},
			{"kly/hr", "Calculate max kly travelled per hour. Arguments: <jump range> [optional: time per jump in seconds (default 45s)] [optional: effiency (default 95)]"},
		}, nil, false)
}

func (s *science) handleCommand(m *dispatch.Message) bool {
	switch m.Command {
	case "g":
		handleGravity(m)
	case "kly/hr":
		handleKlyPerHour(m)
	case "bearing":
		handleBearingAndDistance(m)
	default:
		return false
	}
	return true
}

func (*science) CommandGroup() string {
	return "Elite: Dangerous"
}

func handleBearingAndDistance(m *dispatch.Message) {
	var radius float64
	var start, end LatLong
	var err error
	switch len(m.Args) {
	case 5:
		radius, err = strconv.ParseFloat(m.Args[4], 64)
		if err != nil {
			m.ReplyToChannel("The radius must be a number.")
			return
		}
		fallthrough

	case 4:
		lat1, err1 := strconv.ParseFloat(m.Args[0], 64)
		lon1, err2 := strconv.ParseFloat(m.Args[1], 64)
		lat2, err3 := strconv.ParseFloat(m.Args[2], 64)
		lon2, err4 := strconv.ParseFloat(m.Args[3], 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			m.ReplyToChannel("The coordinates must be a number (lat1 lon1 lat2 lon2 [optional radius in km]).")
			return
		}
		start = NewLatLong(lat1, lon1)
		end = NewLatLong(lat2, lon2)
	default:
		m.ReplyToChannel("Not enough arguments. Expected 4-5 numbers (lat1 lon1 lat2 lon2 [optional radius in km]).")
		return
	}

	bearing, distance := calculateBearingAndDistance(start, end, radius)
	distanceStr := distanceFor(distance)
	m.ReplyToChannel("To get from %s to %s head in bearing **%.1f°**%s.", start, end, bearing, distanceStr)
}

func handleKlyPerHour(m *dispatch.Message) {
	if len(m.Args) < 1 {
		return
	}
	var jumpRange float64
	var jumpTime = 45.0
	var effiency = 95.0
	var err error

	switch len(m.Args) {
	case 3:
		effiency, err = strconv.ParseFloat(m.Args[2], 64)
		if err != nil || effiency <= 0 || effiency >= 100 {
			m.ReplyToChannel("The efficiency must be a number greater than 0 and smaller than 100.")
			return
		}
		fallthrough
	case 2:
		jumpTime, err = strconv.ParseFloat(m.Args[1], 64)
		if err != nil || jumpTime <= 0 {
			m.ReplyToChannel("The time per jump must be a number greater than 0.")
			return
		}
		fallthrough
	case 1:
		jumpRange, err = strconv.ParseFloat(m.Args[0], 64)
		if err != nil || jumpRange <= 0 {
			m.ReplyToChannel("The jump range must be a number greater than 0.")
			return
		}
	default:
		m.ReplyToChannel("Incorrect arguments. Expected: <jump range> [time per jump in seconds] [efficiency]")

	}

	jumpsPerHour := 3600.0 / jumpTime
	avgJump := jumpRange * effiency / 100.0
	rangePerHour := math.Round(jumpsPerHour * avgJump) // Random

	m.ReplyToChannel("Spending an average of `%.0fs` per system, with an average hop of `%.0f` ly (`%.0f%%` efficiency of `%.1f`), you can travel **%.0f ly / hour**.", jumpTime, avgJump, effiency, jumpRange, rangePerHour)
}

type densitySigma struct {
	planetType                                                 string
	densityMin, densityLikelyMin, densityLikelyMax, densityMax float64
}

var densitySigmaArray = []densitySigma{
	{"IW", 1.06E+3, 1.84E+3, 2.62E+3, 3.40E+3},
	{"RIW", 2.25E+3, 2.82E+3, 3.38E+3, 3.95E+3},
	{"RW", 2.94E+3, 3.77E+3, 4.60E+3, 5.43E+3},
	{"HMC", 1.21E+3, 4.60E+3, 8.00E+3, 1.14E+4},
	{"MR", 1.47E+3, 7.99E+3, 1.45E+4, 2.10E+4},
	{"WW", 1.51E+3, 4.24E+3, 6.97E+3, 9.70E+3},
	{"ELW", 4.87E+3, 5.65E+3, 6.43E+3, 7.21E+3},
	{"AW", 4.23E+2, 3.50E+3, 6.59E+3, 9.67E+3},
}

func handleGravity(m *dispatch.Message) {
	if len(m.Args) < 2 {
		m.ReplyToChannel("Missing arguments. Expected: <EarthMasses> <Radius in km>")
		return
	}

	planetMass, err := strconv.ParseFloat(m.Args[0], 64)
	planetRadius, err2 := strconv.ParseFloat(m.Args[1], 64)
	if err != nil || err2 != nil {
		m.ReplyToChannel("The arguments must be numbers. Expected: <EarthMasses> <Radius in km>")
		return
	}

	const G = 6.67e-11
	const earthMass = 5.98e24
	const earthRadius = 6367444.7
	const baseG = G * earthMass / (earthRadius * earthRadius)

	planetG := G * planetMass * earthMass / math.Pow(planetRadius*1000, 2)
	planetDensity := planetMass * earthMass / (4.0 / 3.0 * math.Pi * math.Pow(planetRadius, 3)) * 1e-9 // in SI units of kg/m^3
	var maybeTypes []string
	var likelyTypes []string
	var densityString string
	for _, row := range densitySigmaArray {
		if planetDensity > row.densityLikelyMin && planetDensity < row.densityLikelyMax {
			likelyTypes = append(likelyTypes, row.planetType)
		} else if planetDensity > row.densityMin && planetDensity < row.densityMax {
			maybeTypes = append(maybeTypes, row.planetType)
		}
	}
	if len(likelyTypes) > 0 {
		sort.Strings(likelyTypes)
		densityString += "\n**Likely**: " + strings.Join(likelyTypes, ", ")
	}
	if len(maybeTypes) > 0 {
		sort.Strings(maybeTypes)
		densityString += "\n**Possible**: " + strings.Join(maybeTypes, ", ")
	}
	m.ReplyToChannel("The gravity for a planet with %#.3g Earth Masses and a radius of %.0f km is **%.5g** m/s^2 or **%.5g** g. It has a density of **%.5g** kg/m^3.%s",
		planetMass, planetRadius, planetG, planetG/baseG, planetDensity, densityString)
}

type coord float64

func (c coord) toString(fractionDigits int) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.*f", fractionDigits, c), "0"), ".")
}

func (c coord) toRadians() float64 {
	return float64(c * math.Pi / 180.0)
}
func (c coord) toDegrees() float64 {
	return float64(c * 180.0 / math.Pi)
}

type LatLong struct {
	lat, lon coord
}

func (ll LatLong) String() string {
	return fmt.Sprintf("`(lat %s, lon %s)`", ll.lat.toString(4), ll.lon.toString(4))
}

func NewLatLong(lat, lon float64) LatLong {
	return LatLong{coord(lat), coord(lon)}
}

func calculateBearingAndDistance(start, end LatLong, radius float64) (bearing float64, distance float64) {
	startLonRad := start.lon.toRadians()
	endLonRad := end.lon.toRadians()
	startLatRad := start.lat.toRadians()
	endLatRad := end.lat.toRadians()
	y := math.Sin(endLonRad-startLonRad) * math.Cos(endLatRad)
	x := math.Cos(startLatRad)*math.Sin(endLatRad) - math.Sin(startLatRad)*math.Cos(endLatRad)*math.Cos(endLonRad-startLonRad)
	bearing = coord(math.Atan2(y, x)).toDegrees()

	// We want a value between 0 and 360
	if bearing < 0 {
		bearing += 360
	}

	if radius > 0 {
		deltaLatRad := (end.lat - start.lat).toRadians()
		deltaLonRad := (end.lon - start.lon).toRadians()

		a := math.Sin(deltaLatRad/2)*math.Sin(deltaLatRad/2) + math.Cos(startLatRad)*math.Cos(endLatRad)*math.Sin(deltaLonRad/2)*math.Sin(deltaLonRad/2)
		c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

		distance = radius * c
	}
	return bearing, distance
}

func distanceFor(km float64) string {
	var distance string
	if km > 1 {
		distance = fmt.Sprintf(" for **%.2f km**", km)
	} else if km > 0 {
		distance = fmt.Sprintf(" for **%.1f m**", km*1000.0)
	}
	return distance
}
