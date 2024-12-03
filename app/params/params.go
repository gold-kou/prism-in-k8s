package params

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
)

const (
	defaultTimeout          = 10 * time.Minute
	defaultPrismPort        = 80
	defaultPrismCPU         = "500m"
	defaultPrismMemory      = "512Mi"
	defaultIstioMode        = true
	defaultIstioProxyCPU    = "500m"
	defaultIstioProxyMemory = "512Mi"
)

var (
	errEmptyParameter           = errors.New("empty parameter found")
	errUnsupportedParameterType = errors.New("unsupported parameter type")
	errFailedToOpenConfigFile   = errors.New("failed to open config file")
	errFailedToDecodeConfigFile = errors.New("failed to decode config file")
)

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
	PriorityClassName string
	EcrTags           []ECRTag
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
	PriorityClassName     string        `yaml:"priorityClassName"`
	EcrTags               []ECRTag      `yaml:"ecrTags"`
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
	IstioMode = false
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
