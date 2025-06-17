package highlights_test

import (
	"log"
	"path/filepath"

	"github.com/northbright/highlights"
)

func Example() {
	jsonFile := "examples/good-times-with-maomi-and-mimao/data.json"
	dir := filepath.Dir(jsonFile)

	h, err := highlights.LoadJSON(jsonFile)
	if err != nil {
		log.Printf("highlights.LoadJSON() error: %v", err)
		return
	}

	if err = h.Make(dir); err != nil {
		log.Printf("h.Make() error: %v", err)
	}
	log.Printf("h.Make() succeeded")

	// Output:
}
