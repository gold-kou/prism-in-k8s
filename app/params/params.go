package params

import (
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	defaultTimeout          = 10 * time.Minute
	defaultPrismPort        = 80
	defaultPrismCPU         = "500m"
	defaultPrismMemory      = "512Mi"
	defaultIstioMode        = false
	defaultIstioProxyCPU    = "500m"
	defaultIstioProxyMemory = "512Mi"
)

var validNodeSelectorOperators = map[string]bool{
	"In": true, "NotIn": true, "Exists": true,
	"DoesNotExist": true, "Gt": true, "Lt": true,
}

var (
	// required parameters
	MicroserviceName      string
	MicroserviceNamespace string
	PrismMockSuffix       string
	// optional parameters
	Timeout           time.Duration
	PrismPort         int
	PrismCPU          string
	PrismMemory       string
	IstioMode         bool
	IstioProxyCPU     string
	IstioProxyMemory  string
	PriorityClassName            string
	NodeAffinityMatchExpressions []NodeAffinityMatchExpression
	PodAntiAffinityTopologyKey   string
	EcrTags                      []ECRTag
)

type Config struct {
	MicroserviceName      string        `yaml:"microserviceName"`
	MicroserviceNamespace string        `yaml:"microserviceNamespace"`
	PrismMockSuffix       string        `yaml:"prismMockSuffix"`
	Timeout               time.Duration `yaml:"timeout"`
	PrismPort             int           `yaml:"prismPort"`
	PrismCPU              string        `yaml:"prismCpu"`
	PrismMemory           string        `yaml:"prismMemory"`
	IstioMode             bool          `yaml:"istioMode"`
	IstioProxyCPU         string        `yaml:"istioProxyCpu"`
	IstioProxyMemory      string        `yaml:"istioProxyMemory"`
	PriorityClassName            string                       `yaml:"priorityClassName"`
	NodeAffinityMatchExpressions []NodeAffinityMatchExpression `yaml:"nodeAffinity"`
	PodAntiAffinityTopologyKey   string                       `yaml:"podAntiAffinityTopologyKey"`
	EcrTags                      []ECRTag                     `yaml:"ecrTags"`
}

type NodeAffinityMatchExpression struct {
	Key      string   `yaml:"key"`
	Operator string   `yaml:"operator"`
	Values   []string `yaml:"values"`
}

type ECRTag struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

func init() {
	path := os.Getenv("PARAMS_CONFIG_PATH")
	if path == "" {
		log.Fatal("PARAMS_CONFIG_PATH is not set")
	}
	config, err := LoadConfig(path)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	// required parameters
	MicroserviceName = config.MicroserviceName
	MicroserviceNamespace = config.MicroserviceNamespace
	PrismMockSuffix = config.PrismMockSuffix

	// optional parameters
	Timeout = defaultTimeout
	if config.Timeout != 0 {
		Timeout = config.Timeout
	}
	PrismPort = defaultPrismPort
	if config.PrismPort != 0 {
		PrismPort = config.PrismPort
	}
	PrismCPU = defaultPrismCPU
	if config.PrismCPU != "" {
		PrismCPU = config.PrismCPU
	}
	PrismMemory = defaultPrismMemory
	if config.PrismMemory != "" {
		PrismMemory = config.PrismMemory
	}
	IstioMode = defaultIstioMode
	if config.IstioMode {
		IstioMode = config.IstioMode
	}
	IstioProxyCPU = defaultIstioProxyCPU
	if config.IstioProxyCPU != "" {
		IstioProxyCPU = config.IstioProxyCPU
	}
	IstioProxyMemory = defaultIstioProxyMemory
	if config.IstioProxyMemory != "" {
		IstioProxyMemory = config.IstioProxyMemory
	}
	PriorityClassName = config.PriorityClassName
	NodeAffinityMatchExpressions = config.NodeAffinityMatchExpressions
	PodAntiAffinityTopologyKey = config.PodAntiAffinityTopologyKey
	EcrTags = config.EcrTags
}

func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	return &config, nil
}

func ValidateParams() error {
	params := map[string]interface{}{
		"microserviceName":      MicroserviceName,
		"microserviceNamespace": MicroserviceNamespace,
		"prismMockSuffix":       PrismMockSuffix,
		"timeout":               Timeout,
		"prismPort":             PrismPort,
		"prismCPU":              PrismCPU,
		"prismMemory":           PrismMemory,
		"istioProxyCPU":         IstioProxyCPU,
		"istioProxyMemory":      IstioProxyMemory,
	}

	for name, value := range params {
		switch v := value.(type) {
		case string:
			if v == "" {
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

	for _, expr := range NodeAffinityMatchExpressions {
		if !validNodeSelectorOperators[expr.Operator] {
			return fmt.Errorf("invalid node selector operator: %s", expr.Operator)
		}
	}

	return nil
}
