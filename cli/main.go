package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
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

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go func() {
		<-ctx.Done()
		fmt.Printf("ctx is done\n")
	}()

	if err = h.Make(ctx, dir, os.Stdout, os.Stderr); err != nil {
		fmt.Printf("Make() error: %v\n", err)
		return
	}

	fmt.Printf("Make() succeeded\n")
}
