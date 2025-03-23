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
	switch m.Command {
	case "randomcat":
		go handleRandomCat(m)
	case "randomdog":
		go handleRandomCat(m)
	case "randomkitten":
		m.ReplyToChannel("http://www.randomkittengenerator.com/cats/rotator.php/%d.jpg", time.Now().Nanosecond())
	case "randomcorgi":
		go handleRandomCorgi(m)
	default:
		return false
	}
	return true
}

func (a *animals) HandleCommand(m *dispatch.Message) bool {
	switch len(m.Args) {
	case 0:
		m.ReplyToChannel("I know of the following random images: cat, dog corgi and kitten.")
		return true
	case 1:
		return a.HandlePrefix("random", strings.ToLower(m.Args[0]), m)
	default:
		return false // Only handle empty random
	}
}

func handleRandomCat(m *dispatch.Message) {
	type meowModel struct {
		Id     string `json:"id"`
		Url    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	res, err := http.Get("https://api.thecatapi.com/v1/images/search")
	if err != nil {
		m.ReplyToChannel("Unfortunately, I failed to find any random cats for you today. :-(")
		core.LogError("Failed to get meow: ", err)
		return
	}
	var models []meowModel
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(&models)
	if err != nil || len(models) == 0 || len(models[0].Url) == 0 {
		m.ReplyToChannel("The cats were not parsable today. :-(")
		core.LogError("Failed to parse response", err)
		return
	}
	m.ReplyToChannel(models[0].Url)
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
	m.ReplyToChannel(model.Url)
}

func handleRandomCorgi(m *dispatch.Message) {
	handleRandomDog(m, "corgi")
}
