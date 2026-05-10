package registry_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gold-kou/prism-in-k8s/app/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareBuildContext(t *testing.T) {
	openapiContent := []byte("openapi: 3.0.0\ninfo:\n  title: Test\n  version: 1.0.0\n")
	openapiPath := filepath.Join(t.TempDir(), "openapi.yaml")
	require.NoError(t, os.WriteFile(openapiPath, openapiContent, 0o600))

	buildCtx, err := registry.PrepareBuildContext(openapiPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(buildCtx) })

	expected := map[string][]byte{
		"Dockerfile":              []byte(registry.Dockerfile),
		"openapi.yaml":            openapiContent,
		"openapi-sample.yaml":     []byte(registry.OpenAPISample),
		"empty_check_and_copy.sh": []byte(registry.EmptyCheckScript),
	}

	for name, want := range expected {
		path := filepath.Join(buildCtx, name)
		got, err := os.ReadFile(path)
		require.NoErrorf(t, err, "reading %s", name)
		assert.Equalf(t, want, got, "content of %s", name)

		info, err := os.Stat(path)
		require.NoErrorf(t, err, "stat %s", name)
		assert.Equalf(t, registry.BuildContextFileMode, info.Mode().Perm(), "mode of %s", name)
	}
}

func TestPrepareBuildContext_RelativePath(t *testing.T) {
	openapiContent := []byte("openapi: 3.0.0\n")
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "openapi.yaml"), openapiContent, 0o600))

	t.Chdir(dir)

	buildCtx, err := registry.PrepareBuildContext("openapi.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(buildCtx) })

	got, err := os.ReadFile(filepath.Join(buildCtx, "openapi.yaml"))
	require.NoError(t, err)
	assert.Equal(t, openapiContent, got)
}

func TestPrepareBuildContext_OpenAPIFileNotFound(t *testing.T) {
	tmpRoot := t.TempDir()
	t.Setenv("TMPDIR", tmpRoot)

	buildCtx, err := registry.PrepareBuildContext(filepath.Join(tmpRoot, "missing.yaml"))

	assert.Empty(t, buildCtx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read openapi file")
	assertNoLeakedBuildContext(t, tmpRoot)
}

func TestPrepareBuildContext_OpenAPIPathIsDirectory(t *testing.T) {
	tmpRoot := t.TempDir()
	t.Setenv("TMPDIR", tmpRoot)

	dirAsOpenAPI := filepath.Join(tmpRoot, "openapi-as-dir")
	require.NoError(t, os.Mkdir(dirAsOpenAPI, 0o700))

	buildCtx, err := registry.PrepareBuildContext(dirAsOpenAPI)

	assert.Empty(t, buildCtx)
	require.Error(t, err)
	assertNoLeakedBuildContext(t, tmpRoot)
}

func assertNoLeakedBuildContext(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContainsf(t, e.Name(), "prism-build-", "leaked temp build context: %s", e.Name())
	}
}
