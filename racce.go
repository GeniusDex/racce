package main

import (
	"encoding/json"
	"io/ioutil"
	"log"

	"github.com/geniusdex/racce/accresults"
	"github.com/geniusdex/racce/accserver"
	"github.com/geniusdex/racce/frontend"
)

type configuration struct {
	Frontend frontend.Configuration  `json:"frontend"`
	Results  accresults.Options      `json:"results"`
	Server   accserver.Configuration `json:"server"`
}

func (c *configuration) makeDatabaseConfiguration() *accresults.Configuration {
	return &accresults.Configuration{
		ResultsDir:   c.Server.ResolveResultsDir(),
		NewFileDelay: c.Server.NewResultsDelay,
		Options:      c.Results,
	}
}

func loadConfiguration(filename string) (*configuration, error) {
	fileContents, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var config configuration
	if err = json.Unmarshal(fileContents, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func main() {
	log.Printf("Reading configuration...")
	config, err := loadConfiguration("configuration.json")
	if err != nil {
		log.Panic(err)
	}

	server, err := accserver.NewServer(&config.Server)
	if err != nil {
		log.Printf("Server cannot be managed: %v", err)
	}

	log.Printf("Populating database...")
	db, err := accresults.LoadDatabase(config.makeDatabaseConfiguration())
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Starting frontend...")
	log.Panic(frontend.Run(&config.Frontend, db, server))
}
