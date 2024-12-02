package params

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	PriorityClassName  string
	Timeout            time.Duration
	EcrTags            []ECRTag
	DeploymentAffinity *Affinity
)

type Config struct {
	MicroserviceName      string        `yaml:"microserviceName"`
	MicroserviceNamespace string        `yaml:"microserviceNamespace"`
	PrismMockSuffix       string        `yaml:"prismMockSuffix"`
	PrismPort             int           `yaml:"prismPort"`
	PrismCPU              string        `yaml:"prismCpu"`
	PrismMemory           string        `yaml:"prismMemory"`
	IstioProxyCPU         string        `yaml:"istioProxyCpu"`
	IstioProxyMemory      string        `yaml:"istioProxyMemory"`
	PriorityClassName     string        `yaml:"priorityClassName,omitempty"`
	Timeout               time.Duration `yaml:"timeout"`
	EcrTags               []ECRTag      `yaml:"ecrTags,omitempty"`
	Affinity              *Affinity     `yaml:"affinity,omitempty"`
}

type ECRTag struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

type Affinity struct {
	NodeAffinity    *NodeAffinity    `yaml:"nodeAffinity,omitempty"`
	PodAffinity     *PodAffinity     `yaml:"podAffinity,omitempty"`
	PodAntiAffinity *PodAntiAffinity `yaml:"podAntiAffinity,omitempty"`
}

type NodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []NodeSelectorTerm        `yaml:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `yaml:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

type PodAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `yaml:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `yaml:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

type PodAntiAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm         `yaml:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `yaml:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement `yaml:"matchExpressions,omitempty"`
}

type NodeSelectorRequirement struct {
	Key      string                      `yaml:"key"`
	Operator corev1.NodeSelectorOperator `yaml:"operator"`
	Values   []string                    `yaml:"values,omitempty"`
}

type PreferredSchedulingTerm struct {
	Weight     int32            `yaml:"weight"`
	Preference NodeSelectorTerm `yaml:"preference"`
}

type PodAffinityTerm struct {
	LabelSelector *LabelSelector `yaml:"labelSelector,omitempty"`
	TopologyKey   string         `yaml:"topologyKey"`
}

type WeightedPodAffinityTerm struct {
	Weight          int32           `yaml:"weight"`
	PodAffinityTerm PodAffinityTerm `yaml:"podAffinityTerm"`
}

type LabelSelector struct {
	MatchExpressions []LabelSelectorRequirement `yaml:"matchExpressions,omitempty"`
}

type LabelSelectorRequirement struct {
	Key      string                       `yaml:"key"`
	Operator metav1.LabelSelectorOperator `yaml:"operator"`
	Values   []string                     `yaml:"values,omitempty"`
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
	fmt.Println("debug")
	fmt.Println(config.Affinity)
	DeploymentAffinity = config.Affinity
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
