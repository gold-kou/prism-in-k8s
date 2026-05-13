package keda_test

import (
	"testing"

	"github.com/gold-kou/prism-in-k8s/app/keda"
	"github.com/gold-kou/prism-in-k8s/app/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildScaledObject(t *testing.T) {
	cfg := &params.Config{
		KedaCronTimezone:    "Asia/Tokyo",
		KedaCronStart:       "0 9 * * 1-5",
		KedaCronEnd:         "0 21 * * 1-5",
		KedaDesiredReplicas: "1",
		KedaCPUUtilization:  "50",
		KedaMinReplicas:     "0",
		KedaMaxReplicas:     "1",
	}

	resourceName := "test-service-prism-mock"

	scaledObject := keda.BuildScaledObject(cfg, resourceName)
	require.NotNil(t, scaledObject)

	// verify top-level fields
	assert.Equal(t, "keda.sh/v1alpha1", scaledObject.Object["apiVersion"])
	assert.Equal(t, "ScaledObject", scaledObject.Object["kind"])

	// verify metadata
	metadata, ok := scaledObject.Object["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, resourceName, metadata["name"])

	// verify spec
	spec, ok := scaledObject.Object["spec"].(map[string]interface{})
	require.True(t, ok)

	scaleTargetRef, ok := spec["scaleTargetRef"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, resourceName, scaleTargetRef["name"])

	assert.Equal(t, int64(0), spec["minReplicaCount"])
	assert.Equal(t, int64(1), spec["maxReplicaCount"])

	// verify triggers
	triggers, ok := spec["triggers"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, triggers, 2)

	// CPU trigger (index 0)
	cpuTrigger := triggers[0]
	assert.Equal(t, "cpu", cpuTrigger["type"])
	assert.Equal(t, "Utilization", cpuTrigger["metricType"])
	cpuMetadata, ok := cpuTrigger["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "50", cpuMetadata["value"])

	// Cron trigger (index 1)
	cronTrigger := triggers[1]
	assert.Equal(t, "cron", cronTrigger["type"])

	cronMetadata, ok := cronTrigger["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Asia/Tokyo", cronMetadata["timezone"])
	assert.Equal(t, "0 9 * * 1-5", cronMetadata["start"])
	assert.Equal(t, "0 21 * * 1-5", cronMetadata["end"])
	assert.Equal(t, "1", cronMetadata["desiredReplicas"])
}

func TestBuildScaledObject_MaxReplicasMatchesDesired(t *testing.T) {
	cfg := &params.Config{
		KedaCronTimezone:    "Asia/Tokyo",
		KedaCronStart:       "0 9 * * 1-5",
		KedaCronEnd:         "0 21 * * 1-5",
		KedaDesiredReplicas: "3",
		KedaCPUUtilization:  "50",
		KedaMinReplicas:     "0",
		KedaMaxReplicas:     "1",
	}

	scaledObject := keda.BuildScaledObject(cfg, "test-resource")
	spec, ok := scaledObject.Object["spec"].(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, int64(3), spec["maxReplicaCount"], "maxReplicaCount should be raised to desiredReplicas when desired > max")
}

func TestBuildScaledObject_CustomMinMaxReplicas(t *testing.T) {
	cfg := &params.Config{
		KedaCronTimezone:    "Asia/Tokyo",
		KedaCronStart:       "0 9 * * 1-5",
		KedaCronEnd:         "0 21 * * 1-5",
		KedaDesiredReplicas: "2",
		KedaCPUUtilization:  "50",
		KedaMinReplicas:     "1",
		KedaMaxReplicas:     "5",
	}

	scaledObject := keda.BuildScaledObject(cfg, "test-resource")
	spec, ok := scaledObject.Object["spec"].(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, int64(1), spec["minReplicaCount"], "minReplicaCount should reflect KedaMinReplicas")
	assert.Equal(t, int64(5), spec["maxReplicaCount"], "maxReplicaCount should reflect KedaMaxReplicas when desired < max")
}

func TestBuildScaledObject_CustomCPUUtilization(t *testing.T) {
	cfg := &params.Config{
		KedaCronTimezone:    "Asia/Tokyo",
		KedaCronStart:       "0 9 * * 1-5",
		KedaCronEnd:         "0 21 * * 1-5",
		KedaDesiredReplicas: "1",
		KedaCPUUtilization:  "75",
		KedaMinReplicas:     "0",
		KedaMaxReplicas:     "1",
	}

	scaledObject := keda.BuildScaledObject(cfg, "test-resource")
	spec, ok := scaledObject.Object["spec"].(map[string]interface{})
	require.True(t, ok)

	triggers, ok := spec["triggers"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, triggers, 2)

	cpuMetadata, ok := triggers[0]["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "75", cpuMetadata["value"], "CPU value should reflect configured KedaCPUUtilization")
}
