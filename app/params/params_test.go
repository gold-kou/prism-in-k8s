package params_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gold-kou/prism-in-k8s/app/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	content := `
microserviceName: "sample"
microserviceNamespace: "sample-ns"
prismMockSuffix: "-prism-mock"
prismPort: 8080
istioMode: true
dockerBuildPlatform: "linux/arm64"
virtualServiceRoutes:
  - name: "route1"
    uriPrefix: "/api/"
    method: "GET"
ecrTags:
  - key: "Env"
    value: "prod"
`
	path := filepath.Join(t.TempDir(), "params.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	cfg, err := params.LoadConfig(path)
	require.NoError(t, err)

	// explicit values preserved
	assert.Equal(t, "sample", cfg.MicroserviceName)
	assert.Equal(t, "sample-ns", cfg.MicroserviceNamespace)
	assert.Equal(t, "-prism-mock", cfg.PrismMockSuffix)
	assert.Equal(t, 8080, cfg.PrismPort)
	assert.True(t, cfg.IstioMode)
	assert.Equal(t, "linux/arm64", cfg.DockerBuildPlatform)
	require.Len(t, cfg.VirtualServiceRoutes, 1)
	assert.Equal(t, "route1", cfg.VirtualServiceRoutes[0].Name)
	require.Len(t, cfg.EcrTags, 1)
	assert.Equal(t, "Env", cfg.EcrTags[0].Key)

	// defaults applied for unspecified optional fields
	assert.Equal(t, 10*time.Minute, cfg.Timeout)
	assert.Equal(t, "500m", cfg.PrismCPU)
	assert.Equal(t, "512Mi", cfg.PrismMemory)
	assert.Equal(t, "500m", cfg.IstioProxyCPU)
	assert.Equal(t, "512Mi", cfg.IstioProxyMemory)
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := params.LoadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open config file")
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "params.yaml")
	require.NoError(t, os.WriteFile(path, []byte("invalid: [yaml: content"), 0o600))

	_, err := params.LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode config file")
}

func TestLoadConfig_TypeMismatch(t *testing.T) {
	// prismPort is an int field; supplying a non-numeric string must fail
	// at decode time rather than silently producing a zero value.
	content := "prismPort: not-a-number\n"
	path := filepath.Join(t.TempDir(), "params.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	_, err := params.LoadConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode config file")
}

func TestApplyDefaults_OnEmptyConfig(t *testing.T) {
	cfg := &params.Config{}
	cfg.ApplyDefaults()

	assert.Equal(t, 10*time.Minute, cfg.Timeout)
	assert.Equal(t, 80, cfg.PrismPort)
	assert.Equal(t, "500m", cfg.PrismCPU)
	assert.Equal(t, "512Mi", cfg.PrismMemory)
	assert.Equal(t, "500m", cfg.IstioProxyCPU)
	assert.Equal(t, "512Mi", cfg.IstioProxyMemory)
	assert.Equal(t, "linux/amd64", cfg.DockerBuildPlatform)
	// fields without defaults stay zero
	assert.Empty(t, cfg.MicroserviceName)
	assert.False(t, cfg.IstioMode)
}

func TestApplyDefaults_PreservesExplicitValues(t *testing.T) {
	cfg := &params.Config{
		Timeout:             5 * time.Minute,
		PrismPort:           9090,
		PrismCPU:            "1000m",
		PrismMemory:         "1Gi",
		IstioProxyCPU:       "250m",
		IstioProxyMemory:    "256Mi",
		DockerBuildPlatform: "linux/arm64",
	}
	cfg.ApplyDefaults()

	assert.Equal(t, 5*time.Minute, cfg.Timeout)
	assert.Equal(t, 9090, cfg.PrismPort)
	assert.Equal(t, "1000m", cfg.PrismCPU)
	assert.Equal(t, "1Gi", cfg.PrismMemory)
	assert.Equal(t, "250m", cfg.IstioProxyCPU)
	assert.Equal(t, "256Mi", cfg.IstioProxyMemory)
	assert.Equal(t, "linux/arm64", cfg.DockerBuildPlatform)
}

func TestApplyDefaults_Idempotent(t *testing.T) {
	cfg := &params.Config{}
	cfg.ApplyDefaults()
	first := *cfg
	cfg.ApplyDefaults()
	assert.Equal(t, first, *cfg)
}

func TestValidate_HappyPath(t *testing.T) {
	cfg := validConfig()
	require.NoError(t, cfg.Validate())
}

func TestValidate_Errors(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*params.Config)
		errSubstr string
	}{
		{
			name:      "missing microserviceName",
			mutate:    func(c *params.Config) { c.MicroserviceName = "" },
			errSubstr: "empty parameter found: microserviceName",
		},
		{
			name:      "missing microserviceNamespace",
			mutate:    func(c *params.Config) { c.MicroserviceNamespace = "" },
			errSubstr: "empty parameter found: microserviceNamespace",
		},
		{
			name:      "missing prismMockSuffix",
			mutate:    func(c *params.Config) { c.PrismMockSuffix = "" },
			errSubstr: "empty parameter found: prismMockSuffix",
		},
		{
			name:      "invalid dockerBuildPlatform",
			mutate:    func(c *params.Config) { c.DockerBuildPlatform = "linux/amd86" },
			errSubstr: "invalid dockerBuildPlatform",
		},
		{
			name: "invalid nodeAffinity operator",
			mutate: func(c *params.Config) {
				c.NodeAffinityMatchExpressions = []params.NodeAffinityMatchExpression{
					{Key: "k", Operator: "BadOp"},
				}
			},
			errSubstr: "invalid node selector operator: BadOp",
		},
		{
			name: "virtualServiceRoutes set but istioMode is false",
			mutate: func(c *params.Config) {
				c.IstioMode = false
				c.VirtualServiceRoutes = []params.VirtualServiceRoute{{Name: "r1"}}
			},
			errSubstr: "virtualServiceRoutes can only be set when istioMode is true",
		},
		{
			name: "virtualServiceRoute missing name",
			mutate: func(c *params.Config) {
				c.IstioMode = true
				c.VirtualServiceRoutes = []params.VirtualServiceRoute{{Name: ""}}
			},
			errSubstr: "virtualServiceRoutes[0].name",
		},
		{
			name: "duplicate virtualServiceRoute name",
			mutate: func(c *params.Config) {
				c.IstioMode = true
				c.VirtualServiceRoutes = []params.VirtualServiceRoute{
					{Name: "r1"},
					{Name: "r1"},
				}
			},
			errSubstr: "duplicate virtualServiceRoutes name: r1",
		},
		{
			name: "invalid HTTP method on virtualServiceRoute",
			mutate: func(c *params.Config) {
				c.IstioMode = true
				c.VirtualServiceRoutes = []params.VirtualServiceRoute{
					{Name: "r1", Method: "FOO"},
				}
			},
			errSubstr: "is invalid HTTP method: FOO",
		},
		{
			name: "negative delayNanos on virtualServiceRoute",
			mutate: func(c *params.Config) {
				c.IstioMode = true
				c.VirtualServiceRoutes = []params.VirtualServiceRoute{
					{Name: "r1", DelayNanos: -1},
				}
			},
			errSubstr: "delayNanos must be >= 0",
		},
		{
			name: "delayPercentage above 100 on virtualServiceRoute",
			mutate: func(c *params.Config) {
				c.IstioMode = true
				c.VirtualServiceRoutes = []params.VirtualServiceRoute{
					{Name: "r1", DelayPercentage: 101},
				}
			},
			errSubstr: "delayPercentage must be within [0, 100]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(cfg)
			err := cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errSubstr)
		})
	}
}

// validConfig returns a Config that passes Validate. Tests mutate one field
// at a time to exercise each validation branch in isolation.
func validConfig() *params.Config {
	return &params.Config{
		MicroserviceName:      "test",
		MicroserviceNamespace: "test-ns",
		PrismMockSuffix:       "-mock",
	}
}
