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
	require.Len(t, triggers, 1)

	trigger := triggers[0]
	assert.Equal(t, "cron", trigger["type"])

	triggerMetadata, ok := trigger["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Asia/Tokyo", triggerMetadata["timezone"])
	assert.Equal(t, "0 9 * * 1-5", triggerMetadata["start"])
	assert.Equal(t, "0 21 * * 1-5", triggerMetadata["end"])
	assert.Equal(t, "1", triggerMetadata["desiredReplicas"])
}

func TestBuildScaledObject_MaxReplicasMatchesDesired(t *testing.T) {
	cfg := &params.Config{
		KedaCronTimezone:    "Asia/Tokyo",
		KedaCronStart:       "0 9 * * 1-5",
		KedaCronEnd:         "0 21 * * 1-5",
		KedaDesiredReplicas: "3",
	}

	scaledObject := keda.BuildScaledObject(cfg, "test-resource")
	spec, ok := scaledObject.Object["spec"].(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, int64(3), spec["maxReplicaCount"], "maxReplicaCount should match desiredReplicas when > 1")
}
