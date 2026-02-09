package handlers

//
// Created by David Hedbor on 2/13/16.
// Copyright (c) 2016 NeoTron. All rights reserved.
//

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"GoBot/core"
	"GoBot/core/dispatch"
)

type animals struct {
	dispatch.NoOpMessageHandler
}

func init() {
	randomHelp := "Show image of random animal. Supports cat, dog, corgi, and kitten. Space between *random* and *animal* is optional."
	dispatch.Register(&animals{},
		[]dispatch.MessageCommand{
			{"random", randomHelp},
		}, []dispatch.MessageCommand{
			{"random", ""},
		}, false)
}

func (*animals) HandlePrefix(prefix, suffix string, m *dispatch.Message) bool {
	switch suffix {
	case "whale", "pikachu":
		go handleRandomImage(m, suffix)
	case "kitten":
		m.ReplyToChannel("http://www.randomkittengenerator.com/cats/rotator.php/%d.jpg", time.Now().Nanosecond())
	case "corgi":
		go handleRandomCorgi(m)
	case "cat", "dog", "bird", "panda", "fox", "kangaroo", "raccoon":
		go handleRandomAnimal(m, suffix, true)
	case "redpanda":
		go handleRandomAnimal(m, "red_panda", false)
	default:
		m.ReplyToChannel("%s is an unknown animal.", suffix)
		return false
	}
	return true
}

func (a *animals) HandleCommand(m *dispatch.Message) bool {
	switch len(m.Args) {
	case 0:
		m.ReplyToChannel("I know of the following random images: cat, dog, corgi, kitten, bird, panda, fox, kangaroo, raccoon, red panda, whale and pikachu.")
		return true
	case 1:
		return a.HandlePrefix("random", strings.ToLower(m.Args[0]), m)
	case 2:
		return a.HandlePrefix("random", strings.ToLower(m.Args[0]+m.Args[1]), m)
	default:
		return false // Only handle empty random
	}
}

func handleRandomImage(m *dispatch.Message, image string) {
	type imageModel struct {
		Url string `json:"link"`
	}
	res, err := http.Get(fmt.Sprintf("https://some-random-api.com/img/%s", image))
	if err != nil {
		m.ReplyToChannel("Unfortunately, I failed to find a random %s for you today. :-(", image)
		core.LogError("Failed to get meow: ", err)
		return
	}
	var model imageModel
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&model)
	if err != nil || len(model.Url) == 0 {
		m.ReplyToChannel("The %s were not parsable today. :-(", image)
		core.LogError("Failed to parse response", err)
		return
	}
	m.ReplyToChannel("%s", model.Url)
}

func handleRandomDog(m *dispatch.Message, breed string) {
	type woofModel struct {
		Url    string `json:"message"`
		Status string `json:"status"`
	}
	url := "https://api.thecatapi.com/v1/images/search"
	if breed != "" {
		url = fmt.Sprintf("https://dog.ceo/api/breed/%s/images/random", breed)
	}
	res, err := http.Get(url)
	if err != nil {
		m.ReplyToChannel("Unfortunately, I failed to find any random cats for you today. :-(")
		core.LogError("Failed to get meow: ", err)
		return
	}
	var model woofModel
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&model)
	if err != nil || model.Status != "success" || len(model.Url) == 0 {
		m.ReplyToChannel("The doggos were not parsable today. :-(")
		core.LogError("Failed to parse response", err)
		return
	}
	m.ReplyToChannel("%s", model.Url)
}

func handleRandomCorgi(m *dispatch.Message) {
	handleRandomDog(m, "corgi")
}

func handleRandomAnimal(m *dispatch.Message, breed string, showFacts bool) {
	type animalModel struct {
		Url  string `json:"image"`
		Fact string `json:"fact"`
	}
	url := fmt.Sprintf("https://some-random-api.com/animal/%s", breed)
	res, err := http.Get(url)
	if err != nil {
		m.ReplyToChannel("Unfortunately, I failed to find a random %s for you today. :-(", breed)
		core.LogError("Failed to get animal: ", err)
		return
	}
	var model animalModel
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&model)
	if err != nil || len(model.Url) == 0 {
		m.ReplyToChannel("The %s were not parsable today. :-(", breed)
		core.LogError("Failed to parse response", err)
		return
	}
	if showFacts {
		m.ReplyToChannel("%s\n\n%s", model.Fact, model.Url)
	} else {
		m.ReplyToChannel("%s", model.Url)
	}
}
