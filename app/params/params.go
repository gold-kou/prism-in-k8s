package params

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
)

var (
	AWSConfig     aws.Config
	AWSAccountID  string
	ResourceName  string
	NamespaceName string
	IsTest        bool
)

const (
	// name
	MicroserviceName      = "" // set your microservice name
	MicroserviceNamespace = "" // set your microservice namespace
	PrismMockSuffix       = "-prism-mock"
	// prism container
	PrismPort   = 80
	PrismCPU    = "1"
	PrismMemory = "1Gi"
	// istio container
	IstioProxyCPU    = "500m"
	IstioProxyMemory = "512Mi"
	// general
	Timeout   = 10 * time.Minute
	EcrTagEnv = "stg" // not required
)

func init() {
	// resource name
	ResourceName = "test-microservice"
	NamespaceName = "test-namespace"
	if MicroserviceName != "" && MicroserviceNamespace != "" {
		ResourceName = MicroserviceName + PrismMockSuffix
		NamespaceName = MicroserviceNamespace + PrismMockSuffix
	}
}

func ValidateParams() error {
	params := map[string]interface{}{
		"microserviceName":      MicroserviceName,
		"microserviceNamespace": MicroserviceNamespace,
		"prismMockSuffix":       PrismMockSuffix,
		"prismPort":             PrismPort,
		"prismCPU":              PrismCPU,
		"prismMemory":           PrismMemory,
		"istioProxyCPU":         IstioProxyCPU,
		"istioProxyMemory":      IstioProxyMemory,
		"timeout":               Timeout,
		"ecrTagEnv":             EcrTagEnv,
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
