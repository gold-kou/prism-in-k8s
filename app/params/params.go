package params

import (
	"errors"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	DefaultTimeout          = 10 * time.Minute
	DefaultPrismPort        = 80
	DefaultPrismCPU         = "500m"
	DefaultPrismMemory      = "512Mi"
	DefaultIstioProxyCPU    = "500m"
	DefaultIstioProxyMemory = "512Mi"
)

var (
	errEmptyParameter              = errors.New("empty parameter found")
	errUnsupportedParameterType    = errors.New("unsupported parameter type")
	errFailedToOpenConfigFile      = errors.New("failed to open config file")
	errFailedToDecodeConfigFile    = errors.New("failed to decode config file")
	errInvalidNodeSelectorOperator = errors.New("invalid node selector operator")
)

var validNodeSelectorOperators = map[string]bool{
	"In": true, "NotIn": true, "Exists": true,
	"DoesNotExist": true, "Gt": true, "Lt": true,
}

type Config struct {
	// required parameters
	MicroserviceName      string `yaml:"microserviceName"`
	MicroserviceNamespace string `yaml:"microserviceNamespace"`
	PrismMockSuffix       string `yaml:"prismMockSuffix"`
	// optional parameters
	Timeout                      time.Duration                 `yaml:"timeout"`
	PrismPort                    int                           `yaml:"prismPort"`
	PrismCPU                     string                        `yaml:"prismCpu"`
	PrismMemory                  string                        `yaml:"prismMemory"`
	IstioMode                    bool                          `yaml:"istioMode"`
	IstioProxyCPU                string                        `yaml:"istioProxyCpu"`
	IstioProxyMemory             string                        `yaml:"istioProxyMemory"`
	PriorityClassName            string                        `yaml:"priorityClassName"`
	NodeAffinityMatchExpressions []NodeAffinityMatchExpression `yaml:"nodeAffinity"`
	PodAntiAffinityTopologyKey   string                        `yaml:"podAntiAffinityTopologyKey"`
	EcrTags                      []ECRTag                      `yaml:"ecrTags"`
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

	config.ApplyDefaults()

	return &config, nil
}

func (c *Config) ApplyDefaults() {
	if c.Timeout == 0 {
		c.Timeout = DefaultTimeout
	}
	if c.PrismPort == 0 {
		c.PrismPort = DefaultPrismPort
	}
	if c.PrismCPU == "" {
		c.PrismCPU = DefaultPrismCPU
	}
	if c.PrismMemory == "" {
		c.PrismMemory = DefaultPrismMemory
	}
	if c.IstioProxyCPU == "" {
		c.IstioProxyCPU = DefaultIstioProxyCPU
	}
	if c.IstioProxyMemory == "" {
		c.IstioProxyMemory = DefaultIstioProxyMemory
	}
}

func (c *Config) Validate() error {
	params := map[string]interface{}{
		"microserviceName":      c.MicroserviceName,
		"microserviceNamespace": c.MicroserviceNamespace,
		"prismMockSuffix":       c.PrismMockSuffix,
		"timeout":               c.Timeout,
		"prismPort":             c.PrismPort,
		"prismCPU":              c.PrismCPU,
		"prismMemory":           c.PrismMemory,
		"istioProxyCPU":         c.IstioProxyCPU,
		"istioProxyMemory":      c.IstioProxyMemory,
	}

	for name, value := range params {
		switch v := value.(type) {
		case string:
			if v == "" {
				return fmt.Errorf("%w: %s", errEmptyParameter, name)
			}
		case int:
			if v == 0 {
				return fmt.Errorf("%w: %s", errEmptyParameter, name)
			}
		case time.Duration:
			if v == 0*time.Millisecond {
				return fmt.Errorf("%w: %s", errEmptyParameter, name)
			}
		default:
			return fmt.Errorf("%w: %s", errUnsupportedParameterType, name)
		}
	}

	for _, expr := range c.NodeAffinityMatchExpressions {
		if !validNodeSelectorOperators[expr.Operator] {
			return fmt.Errorf("%w: %s", errInvalidNodeSelectorOperator, expr.Operator)
		}
	}

	return nil
}
