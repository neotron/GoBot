package handlers

//
// Created by David Hedbor on 2/13/16.
// Copyright (c) 2016 NeoTron. All rights reserved.
//

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"GoBot/core"
	"GoBot/core/dispatch"
)

type animals struct {
	dispatch.NoOpMessageHandler
}

func (*animals) handlePrefix(prefix string, m *dispatch.Message) bool {
	switch m.Command {
	case "randomcat":
		go handleRandomCat(m)
	case "randomdog":
		m.ReplyToChannel("http://www.randomdoggiegenerator.com/randomdoggie.php/%d.jpg", time.Now())
	case "randomkitten":
		m.ReplyToChannel("http://www.randomkittengenerator.com/cats/rotator.php/%s.jpg", time.Now())
	case "randomcorgi":
		go handleRandomCorgi(m)
	default:
		return false
	}
	return true
}

func (a *animals) handleCommand(m *dispatch.Message) bool {
	switch len(m.Args) {
	case 0:
		m.ReplyToChannel("I know of the following random images: cat, dog corgi and kitten.")
		return true
	case 1:
		m.Command += strings.ToLower(m.Args[0])
		return a.handlePrefix(m.Command, m)
	default:
		return false // Only handle empty random
	}
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

type meowModel struct {
	File string
}

func handleRandomCat(m *dispatch.Message) {
	res, err := http.Get("http://aws.random.cat/meow")
	defer res.Body.Close()
	if err != nil {
		m.ReplyToChannel("Unfortunately, I failed to find any random cats for you today. :-(")
		core.LogError("Failed to get meow: ", err)
		return
	}
	decoder := json.NewDecoder(res.Body)
	var model meowModel
	err = decoder.Decode(&model)
	if err != nil || len(model.File) == 0 {
		m.ReplyToChannel("The cats were not parsable today. :-(")
		core.LogError("Failed to parse response", err)
		return
	}
	m.ReplyToChannel(model.File)
}

func handleRandomCorgi(m *dispatch.Message) {
	res, err := http.Head("http://cor.gi/random")
	if err != nil {
		m.ReplyToChannel("Unfortunately, I failed produce a suitable corgy today. :-(")
		core.LogError("Failed to get corgi: ", err)
		return
	}
	m.ReplyToChannel("%v", res.Request.URL)
	//	m.ReplyToChannel(model.File)
}
