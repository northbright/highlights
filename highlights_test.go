package highlights_test

import (
	"context"
	"log"
	"os"
	"os/signal"
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

	// Press Ctrl+C to to make ctx done.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go func() {
		<-ctx.Done()
		log.Printf("ctx is done\n")
	}()

	if err = h.Make(ctx, dir, os.Stdout, os.Stderr); err != nil {
		log.Printf("h.Make() error: %v", err)
		return
	}
	log.Printf("h.Make() succeeded")

	// Output:
}
