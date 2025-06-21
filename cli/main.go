package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/northbright/highlights"
)

func main() {
	jsonFile := ""

	flag.StringVar(&jsonFile, "i", "", "input JSON file.")
	flag.Parse()

	if jsonFile == "" {
		fmt.Printf("empty JSON file\n")
		return
	}

	dir := filepath.Dir(jsonFile)
	fmt.Printf("JSON file = %s\ndefault dir to find input videos: %v\n", jsonFile, dir)

	h, err := highlights.LoadJSON(jsonFile)
	if err != nil {
		fmt.Printf("failed to load JSON: %v\n", err)
		return
	}

	if err = h.Make(dir, os.Stdout, os.Stderr); err != nil {
		log.Printf("h.Make() error: %v", err)
	}
	log.Printf("h.Make() succeeded")
}
