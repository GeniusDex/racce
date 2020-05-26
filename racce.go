package main

import (
	"encoding/json"
	"io/ioutil"
	"log"

	"github.com/geniusdex/racce/accresults"
)

type configurationFrontend struct {
	Listen string `json:"listen"`
}

type configuration struct {
	Frontend   *configurationFrontend `json:"frontend"`
	ResultsDir string                 `json:"resultsDir"`
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
		log.Fatal(err)
	}

	log.Printf("Populating database...")
	db, err := accresults.LoadDatabase(config.ResultsDir)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting server...")
	log.Fatal(RunServer(config.Frontend, db))
}
