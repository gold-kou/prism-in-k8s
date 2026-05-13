package params

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	defaultTimeout             = 10 * time.Minute
	defaultPrismPort           = 80
	defaultPrismCPU            = "500m"
	defaultPrismMemory         = "512Mi"
	defaultIstioProxyCPU       = "500m"
	defaultIstioProxyMemory    = "512Mi"
	defaultDockerBuildPlatform = "linux/amd64"
	defaultKedaCronTimezone    = "Asia/Tokyo"
	defaultKedaCronStart       = "0 9 * * 1-5"
	defaultKedaCronEnd         = "0 21 * * 1-5"
	defaultKedaDesiredReplicas = "1"
	defaultKedaCPUUtilization  = "50"
	defaultKedaMinReplicas     = "0"
	defaultKedaMaxReplicas     = "1"
	maxDelayPercentage         = 100.0
)

var validNodeSelectorOperators = map[string]bool{
	"In": true, "NotIn": true, "Exists": true,
	"DoesNotExist": true, "Gt": true, "Lt": true,
}

var validHTTPMethods = map[string]bool{
	"GET": true, "HEAD": true, "POST": true, "PUT": true,
	"PATCH": true, "DELETE": true, "CONNECT": true,
	"OPTIONS": true, "TRACE": true,
}

var validDockerBuildPlatforms = map[string]bool{
	"linux/amd64": true,
	"linux/arm64": true,
}

type Config struct {
	MicroserviceName             string                        `yaml:"microserviceName"`
	MicroserviceNamespace        string                        `yaml:"microserviceNamespace"`
	PrismMockSuffix              string                        `yaml:"prismMockSuffix"`
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
	VirtualServiceRoutes         []VirtualServiceRoute         `yaml:"virtualServiceRoutes"`
	DockerBuildPlatform          string                        `yaml:"dockerBuildPlatform"`
	KedaMode                     bool                          `yaml:"kedaMode"`
	KedaCronTimezone             string                        `yaml:"kedaCronTimezone"`
	KedaCronStart                string                        `yaml:"kedaCronStart"`
	KedaCronEnd                  string                        `yaml:"kedaCronEnd"`
	KedaDesiredReplicas          string                        `yaml:"kedaDesiredReplicas"`
	KedaCPUUtilization           string                        `yaml:"kedaCpuUtilization"`
	KedaMinReplicas              string                        `yaml:"kedaMinReplicas"`
	KedaMaxReplicas              string                        `yaml:"kedaMaxReplicas"`
}

type VirtualServiceRoute struct {
	Name            string  `yaml:"name"`
	URIPrefix       string  `yaml:"uriPrefix"`
	Method          string  `yaml:"method"`
	DelayNanos      int32   `yaml:"delayNanos"`
	DelayPercentage float64 `yaml:"delayPercentage"`
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
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var cfg Config
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&cfg); err != nil {
		// this error includes type mismatching error
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	cfg.ApplyDefaults()
	return &cfg, nil
}

func (c *Config) ApplyDefaults() {
	if c.Timeout == 0 {
		c.Timeout = defaultTimeout
	}
	if c.PrismPort == 0 {
		c.PrismPort = defaultPrismPort
	}
	if c.PrismCPU == "" {
		c.PrismCPU = defaultPrismCPU
	}
	if c.PrismMemory == "" {
		c.PrismMemory = defaultPrismMemory
	}
	if c.IstioProxyCPU == "" {
		c.IstioProxyCPU = defaultIstioProxyCPU
	}
	if c.IstioProxyMemory == "" {
		c.IstioProxyMemory = defaultIstioProxyMemory
	}
	if c.DockerBuildPlatform == "" {
		c.DockerBuildPlatform = defaultDockerBuildPlatform
	}
	if c.KedaCronTimezone == "" {
		c.KedaCronTimezone = defaultKedaCronTimezone
	}
	if c.KedaCronStart == "" {
		c.KedaCronStart = defaultKedaCronStart
	}
	if c.KedaCronEnd == "" {
		c.KedaCronEnd = defaultKedaCronEnd
	}
	if c.KedaDesiredReplicas == "" {
		c.KedaDesiredReplicas = defaultKedaDesiredReplicas
	}
	if c.KedaCPUUtilization == "" {
		c.KedaCPUUtilization = defaultKedaCPUUtilization
	}
	if c.KedaMinReplicas == "" {
		c.KedaMinReplicas = defaultKedaMinReplicas
	}
	if c.KedaMaxReplicas == "" {
		c.KedaMaxReplicas = defaultKedaMaxReplicas
	}
}

func (c *Config) Validate() error {
	c.ApplyDefaults()

	requiredStrings := map[string]string{
		"microserviceName":      c.MicroserviceName,
		"microserviceNamespace": c.MicroserviceNamespace,
		"prismMockSuffix":       c.PrismMockSuffix,
	}
	for name, v := range requiredStrings {
		if v == "" {
			return fmt.Errorf("empty parameter found: %s", name)
		}
	}

	for _, expr := range c.NodeAffinityMatchExpressions {
		if !validNodeSelectorOperators[expr.Operator] {
			return fmt.Errorf("invalid node selector operator: %s", expr.Operator)
		}
	}

	if !validDockerBuildPlatforms[c.DockerBuildPlatform] {
		return fmt.Errorf("invalid dockerBuildPlatform: %s (must be linux/amd64 or linux/arm64)", c.DockerBuildPlatform)
	}

	if err := c.validateKedaParams(); err != nil {
		return err
	}

	return c.validateVirtualServiceRoutes()
}

func (c *Config) validateKedaParams() error {
	if !c.KedaMode {
		return nil
	}

	minR, err := strconv.ParseInt(c.KedaMinReplicas, 10, 64)
	if err != nil || minR < 0 {
		return fmt.Errorf("kedaMinReplicas must be a non-negative integer: %q", c.KedaMinReplicas)
	}
	maxR, err := strconv.ParseInt(c.KedaMaxReplicas, 10, 64)
	if err != nil || maxR < 1 {
		return fmt.Errorf("kedaMaxReplicas must be a positive integer: %q", c.KedaMaxReplicas)
	}
	desiredR, err := strconv.ParseInt(c.KedaDesiredReplicas, 10, 64)
	if err != nil || desiredR < 1 {
		return fmt.Errorf("kedaDesiredReplicas must be a positive integer: %q", c.KedaDesiredReplicas)
	}
	cpu, err := strconv.ParseInt(c.KedaCPUUtilization, 10, 64)
	if err != nil || cpu < 1 || cpu > 100 {
		return fmt.Errorf("kedaCpuUtilization must be in [1, 100]: %q", c.KedaCPUUtilization)
	}
	if minR > maxR {
		return fmt.Errorf("kedaMinReplicas (%d) must be <= kedaMaxReplicas (%d)", minR, maxR)
	}
	if desiredR > maxR {
		return fmt.Errorf("kedaDesiredReplicas (%d) must be <= kedaMaxReplicas (%d)", desiredR, maxR)
	}

	return nil
}

func (c *Config) validateVirtualServiceRoutes() error {
	if len(c.VirtualServiceRoutes) > 0 && !c.IstioMode {
		return errors.New("virtualServiceRoutes can only be set when istioMode is true")
	}

	seen := make(map[string]struct{}, len(c.VirtualServiceRoutes))
	for i, route := range c.VirtualServiceRoutes {
		if route.Name == "" {
			return fmt.Errorf("empty parameter found: virtualServiceRoutes[%d].name", i)
		}
		if _, ok := seen[route.Name]; ok {
			return fmt.Errorf("duplicate virtualServiceRoutes name: %s", route.Name)
		}
		seen[route.Name] = struct{}{}
		if route.Method != "" && !validHTTPMethods[route.Method] {
			return fmt.Errorf("virtualServiceRoutes[%d].method is invalid HTTP method: %s", i, route.Method)
		}
		if route.DelayNanos < 0 {
			return fmt.Errorf("virtualServiceRoutes[%d].delayNanos must be >= 0: %d", i, route.DelayNanos)
		}
		if route.DelayPercentage < 0 || route.DelayPercentage > maxDelayPercentage {
			return fmt.Errorf("virtualServiceRoutes[%d].delayPercentage must be within [0, 100]: %f", i, route.DelayPercentage)
		}
	}

	return nil
}
