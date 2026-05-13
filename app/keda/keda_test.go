package keda_test

import (
	"testing"

	"github.com/gold-kou/prism-in-k8s/app/keda"
	"github.com/gold-kou/prism-in-k8s/app/params"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTimezone  = "Asia/Tokyo"
	testCronStart = "0 9 * * 1-5"
	testCronEnd   = "0 21 * * 1-5"
)

func TestBuildScaledObject(t *testing.T) {
	cfg := &params.Config{
		KedaCronTimezone:    testTimezone,
		KedaCronStart:       testCronStart,
		KedaCronEnd:         testCronEnd,
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
	metadata, isMap := scaledObject.Object["metadata"].(map[string]interface{})
	require.True(t, isMap)
	assert.Equal(t, resourceName, metadata["name"])

	// verify spec
	spec, isMap := scaledObject.Object["spec"].(map[string]interface{})
	require.True(t, isMap)

	scaleTargetRef, isMap := spec["scaleTargetRef"].(map[string]interface{})
	require.True(t, isMap)
	assert.Equal(t, resourceName, scaleTargetRef["name"])

	assert.Equal(t, int64(0), spec["minReplicaCount"])
	assert.Equal(t, int64(1), spec["maxReplicaCount"])

	// verify triggers
	triggers, isSlice := spec["triggers"].([]map[string]interface{})
	require.True(t, isSlice)
	require.Len(t, triggers, 2)

	// CPU trigger (index 0)
	cpuTrigger := triggers[0]
	assert.Equal(t, "cpu", cpuTrigger["type"])
	assert.Equal(t, "Utilization", cpuTrigger["metricType"])
	cpuMetadata, isMap := cpuTrigger["metadata"].(map[string]interface{})
	require.True(t, isMap)
	assert.Equal(t, "50", cpuMetadata["value"])

	// Cron trigger (index 1)
	cronTrigger := triggers[1]
	assert.Equal(t, "cron", cronTrigger["type"])

	cronMetadata, isMap := cronTrigger["metadata"].(map[string]interface{})
	require.True(t, isMap)
	assert.Equal(t, testTimezone, cronMetadata["timezone"])
	assert.Equal(t, testCronStart, cronMetadata["start"])
	assert.Equal(t, testCronEnd, cronMetadata["end"])
	assert.Equal(t, "1", cronMetadata["desiredReplicas"])
}

func TestBuildScaledObject_DesiredAndMaxEqual(t *testing.T) {
	// Validate() guarantees desired <= max, so callers can simply set
	// KedaMaxReplicas to match KedaDesiredReplicas when desired > 1.
	cfg := &params.Config{
		KedaCronTimezone:    testTimezone,
		KedaCronStart:       testCronStart,
		KedaCronEnd:         testCronEnd,
		KedaDesiredReplicas: "3",
		KedaCPUUtilization:  "50",
		KedaMinReplicas:     "0",
		KedaMaxReplicas:     "3",
	}

	scaledObject := keda.BuildScaledObject(cfg, "test-resource")
	spec, isMap := scaledObject.Object["spec"].(map[string]interface{})
	require.True(t, isMap)

	assert.Equal(t, int64(3), spec["maxReplicaCount"])
}

func TestBuildScaledObject_CustomMinMaxReplicas(t *testing.T) {
	cfg := &params.Config{
		KedaCronTimezone:    testTimezone,
		KedaCronStart:       testCronStart,
		KedaCronEnd:         testCronEnd,
		KedaDesiredReplicas: "2",
		KedaCPUUtilization:  "50",
		KedaMinReplicas:     "1",
		KedaMaxReplicas:     "5",
	}

	scaledObject := keda.BuildScaledObject(cfg, "test-resource")
	spec, isMap := scaledObject.Object["spec"].(map[string]interface{})
	require.True(t, isMap)

	assert.Equal(t, int64(1), spec["minReplicaCount"], "minReplicaCount should reflect KedaMinReplicas")
	assert.Equal(t, int64(5), spec["maxReplicaCount"], "maxReplicaCount should reflect KedaMaxReplicas when desired < max")
}

func TestBuildScaledObject_CustomCPUUtilization(t *testing.T) {
	cfg := &params.Config{
		KedaCronTimezone:    testTimezone,
		KedaCronStart:       testCronStart,
		KedaCronEnd:         testCronEnd,
		KedaDesiredReplicas: "1",
		KedaCPUUtilization:  "75",
		KedaMinReplicas:     "0",
		KedaMaxReplicas:     "1",
	}

	scaledObject := keda.BuildScaledObject(cfg, "test-resource")
	spec, isMap := scaledObject.Object["spec"].(map[string]interface{})
	require.True(t, isMap)

	triggers, isSlice := spec["triggers"].([]map[string]interface{})
	require.True(t, isSlice)
	require.Len(t, triggers, 2)

	cpuMetadata, isMap := triggers[0]["metadata"].(map[string]interface{})
	require.True(t, isMap)
	assert.Equal(t, "75", cpuMetadata["value"], "CPU value should reflect configured KedaCPUUtilization")
}
