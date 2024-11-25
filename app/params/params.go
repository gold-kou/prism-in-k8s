package params

import (
	_ "embed"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

var (
	errEmptyParameter           = errors.New("empty parameter found")
	errUnsupportedParameterType = errors.New("unsupported parameter type")
	errFailedToOpenConfigFile   = errors.New("failed to open config file")
	errFailedToDecodeConfigFile = errors.New("failed to decode config file")
)

var (
	// name
	MicroserviceName      string
	MicroserviceNamespace string
	PrismMockSuffix       string
	// prism container
	PrismPort   int
	PrismCPU    string
	PrismMemory string
	// istio container
	IstioProxyCPU    string
	IstioProxyMemory string
	// others
	PriorityClassName string
	Timeout           time.Duration
	EcrTags           []ECRTag
)

type Config struct {
	MicroserviceName      string        `yaml:"microservice_name"`
	MicroserviceNamespace string        `yaml:"microservice_namespace"`
	PrismMockSuffix       string        `yaml:"prism_mock_suffix"`
	PrismPort             int           `yaml:"prism_port"`
	PrismCPU              string        `yaml:"prism_cpu"`
	PrismMemory           string        `yaml:"prism_memory"`
	IstioProxyCPU         string        `yaml:"istio_proxy_cpu"`
	IstioProxyMemory      string        `yaml:"istio_proxy_memory"`
	PriorityClassName     string        `yaml:"priority_class_name"`
	Timeout               time.Duration `yaml:"timeout"`
	EcrTags               []ECRTag      `yaml:"ecr_tags"`
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

	MicroserviceName = config.MicroserviceName
	MicroserviceNamespace = config.MicroserviceNamespace
	PrismMockSuffix = config.PrismMockSuffix
	PrismPort = config.PrismPort
	PrismCPU = config.PrismCPU
	PrismMemory = config.PrismMemory
	IstioProxyCPU = config.IstioProxyCPU
	IstioProxyMemory = config.IstioProxyMemory
	PriorityClassName = config.PriorityClassName
	Timeout = config.Timeout
	EcrTags = config.EcrTags
}

func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errFailedToOpenConfigFile, err)
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("%w: %w", errFailedToDecodeConfigFile, err)
	}

	return &config, nil
}

// func LoadConfig(data []byte) (*Config, error) {
// 	var config Config
// 	if err := yaml.Unmarshal(data, &config); err != nil {
// 		return nil, fmt.Errorf("failed to decode config data: %w", err)
// 	}
// 	return &config, nil
// }

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
	}

	for name, value := range params {
		switch v := value.(type) {
		case string:
			if v == "" {
				return xerrors.Errorf("%w: %s", errEmptyParameter, name)
			}
		case int:
			if v == 0 {
				return xerrors.Errorf("%w: %s", errEmptyParameter, name)
			}
		case time.Duration:
			if v == 0*time.Millisecond {
				return xerrors.Errorf("%w: %s", errEmptyParameter, name)
			}
		default:
			return xerrors.Errorf("%w: %s", errUnsupportedParameterType, name)
		}
	}
	return nil
}
