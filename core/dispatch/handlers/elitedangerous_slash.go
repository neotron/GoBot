package handlers

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var eliteDangerousSlashCommands = []*discordgo.ApplicationCommand{
	{
		Name:        "g",
		Description: "Calculate gravity for a planet",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionNumber,
				Name:        "mass",
				Description: "Planet mass in Earth masses",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionNumber,
				Name:        "radius",
				Description: "Planet radius in km",
				Required:    true,
			},
		},
	},
}

// GetEliteDangerousSlashCommands returns the E:D slash commands for combined registration
func GetEliteDangerousSlashCommands() []*discordgo.ApplicationCommand {
	return eliteDangerousSlashCommands
}

// HandleEliteDangerousSlashCommand handles Elite Dangerous slash command interactions
func HandleEliteDangerousSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) bool {
	if i.Type != discordgo.InteractionApplicationCommand {
		return false
	}

	data := i.ApplicationCommandData()

	switch data.Name {
	case "g":
		handleGravitySlash(s, i, data)
		return true
	default:
		return false
	}
}

func handleGravitySlash(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ApplicationCommandInteractionData) {
	planetMass := data.Options[0].FloatValue()
	planetRadius := data.Options[1].FloatValue()

	if planetMass <= 0 || planetRadius <= 0 {
		respondEphemeral(s, i, "Mass and radius must be greater than 0")
		return
	}

	const G = 6.67e-11
	const earthMass = 5.98e24
	const earthRadius = 6367444.7
	const baseG = G * earthMass / (earthRadius * earthRadius)

	planetG := G * planetMass * earthMass / math.Pow(planetRadius*1000, 2)
	planetDensity := planetMass * earthMass / (4.0 / 3.0 * math.Pi * math.Pow(planetRadius, 3)) * 1e-9

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

	response := fmt.Sprintf("The gravity for a planet with %#.3g Earth Masses and a radius of %.0f km is **%.5g** m/s² or **%.5g** g. It has a density of **%.5g** kg/m³.%s",
		planetMass, planetRadius, planetG, planetG/baseG, planetDensity, densityString)

	respondEphemeral(s, i, response)
}

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}
