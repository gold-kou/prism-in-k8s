package main

import (
	"fmt"
	"time"
)

const (
	// name
	microserviceName      = "" // set your microservice name
	microserviceNamespace = "" // set your microservice namespace
	prismMockSuffix       = "-prism-mock"
	// prism container
	prismPort   = 80
	prismCPU    = "1"
	prismMemory = "1Gi"
	// istio container
	istioProxyCPU    = "500m"
	istioProxyMemory = "512Mi"
	// general
	timeout   = 10 * time.Minute
	ecrTagEnv = "stg" // not required
)

func validateParams() error {
	params := map[string]interface{}{
		"microserviceName":      microserviceName,
		"microserviceNamespace": microserviceNamespace,
		"prismMockSuffix":       prismMockSuffix,
		"prismPort":             prismPort,
		"prismCPU":              prismCPU,
		"prismMemory":           prismMemory,
		"istioProxyCPU":         istioProxyCPU,
		"istioProxyMemory":      istioProxyMemory,
		"timeout":               timeout,
		"ecrTagEnv":             ecrTagEnv,
	}

	for name, value := range params {
		switch v := value.(type) {
		case string:
			if v == "" && name != "ecrTagEnv" {
				return fmt.Errorf("empty parameter found: %s", name)
			}
		case int:
			if v == 0 {
				return fmt.Errorf("empty parameter found: %s", name)
			}
		case time.Duration:
			if v == 0*time.Millisecond {
				return fmt.Errorf("empty parameter found: %s", name)
			}
		default:
			return fmt.Errorf("unsupported parameter type: %s", name)
		}
	}

	return nil
}
