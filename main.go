package main

import (
	"log"

	"github.com/gold-kou/prism-in-k8s/app"
)

func main() {
	a, err := app.NewApp()
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}
	a.Run()
}
