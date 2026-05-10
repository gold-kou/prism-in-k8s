package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/gold-kou/prism-in-k8s/app"
	"github.com/gold-kou/prism-in-k8s/app/params"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	isCreate := flag.Bool("create", false, "set to true if running in create mode")
	isDelete := flag.Bool("delete", false, "set to true if running in delete mode")
	isTest := flag.Bool("test", false, "set to true if running in test mode")
	flag.Parse()

	paramsPath := os.Getenv("PARAMS_CONFIG_PATH")
	if paramsPath == "" {
		paramsPath = "./params.yaml"
	}
	cfg, err := params.LoadConfig(paramsPath)
	if err != nil {
		return fmt.Errorf("failed to load params: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	openapiPath := os.Getenv("OPENAPI_PATH")
	if openapiPath == "" {
		openapiPath = "./openapi.yaml"
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	a, err := app.NewApp(ctx, cfg, openapiPath, *isTest)
	if err != nil {
		return fmt.Errorf("failed to initialize app: %w", err)
	}

	switch {
	case *isCreate:
		if err := a.Create(ctx); err != nil {
			return fmt.Errorf("create failed: %w", err)
		}
	case *isDelete:
		if err := a.Delete(ctx); err != nil {
			return fmt.Errorf("delete failed: %w", err)
		}
	}
	return nil
}
