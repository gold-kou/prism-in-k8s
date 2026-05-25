package keda

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gold-kou/prism-in-k8s/app/params"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	restclient "k8s.io/client-go/rest"
)

const metadataKey = "metadata"

var scaledObjectGVR = schema.GroupVersionResource{
	Group:    "keda.sh",
	Version:  "v1alpha1",
	Resource: "scaledobjects",
}

func CreateKedaResources(ctx context.Context, cfg *params.Config, kubeconfig *restclient.Config, namespaceName, resourceName string) error {
	dynamicClient, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Fail fast when the API server returns 404 for the ScaledObject resource.
	// dynamic client talks to the API server directly via the GVR, so a missing
	// CRD surfaces as apierrors.IsNotFound here (not as a client-side
	// RESTMapper error). A missing namespace would also produce 404, but in
	// this call path the namespace is created beforehand by k8s.CreateK8sResources,
	// so a 404 most likely means KEDA itself is not installed. Permission or
	// transient errors are deferred to the Create call below.
	if _, err := dynamicClient.Resource(scaledObjectGVR).Namespace(namespaceName).List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("KEDA ScaledObject CRD or namespace %q not found; kedaMode requires KEDA Operator installed in the cluster: %w", namespaceName, err)
		}
		slog.Warn("Failed to list ScaledObject during pre-flight check; deferring to Create", "namespace", namespaceName, "error", err)
	}

	scaledObject := BuildScaledObject(cfg, resourceName)

	_, err = dynamicClient.Resource(scaledObjectGVR).Namespace(namespaceName).Create(ctx, scaledObject, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create ScaledObject %s/%s: %w", namespaceName, resourceName, err)
		}
		slog.Warn("The ScaledObject already exists", "namespace", namespaceName, "resourceName", resourceName)
	} else {
		slog.Info("ScaledObject is created successfully", "namespace", namespaceName, "resourceName", resourceName)
	}
	return nil
}

func DeleteKedaResources(ctx context.Context, kubeconfig *restclient.Config, namespaceName, resourceName string) error {
	dynamicClient, err := dynamic.NewForConfig(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	err = dynamicClient.Resource(scaledObjectGVR).Namespace(namespaceName).Delete(ctx, resourceName, metav1.DeleteOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete ScaledObject %s/%s: %w", namespaceName, resourceName, err)
		}
		slog.Warn("The ScaledObject is not found", "namespace", namespaceName, "resourceName", resourceName)
	} else {
		slog.Info("ScaledObject is deleted successfully", "namespace", namespaceName, "resourceName", resourceName)
	}
	return nil
}

// BuildScaledObject constructs the ScaledObject manifest from cfg.
// Caller must have run cfg.Validate() so that KEDA numeric params are well-formed
// and the invariants (min <= max, desired <= max) hold.
func BuildScaledObject(cfg *params.Config, resourceName string) *unstructured.Unstructured {
	// KEDA trigger metadata expects string values, so convert the int param at the boundary.
	desiredReplicas := strconv.Itoa(cfg.KedaDesiredReplicas)

	scaledObject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "keda.sh/v1alpha1",
			"kind":       "ScaledObject",
			metadataKey: map[string]interface{}{
				"name": resourceName,
			},
			"spec": map[string]interface{}{
				"scaleTargetRef": map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"name":       resourceName,
				},
				"minReplicaCount": int64(cfg.KedaMinReplicas),
				"maxReplicaCount": int64(cfg.KedaMaxReplicas),
				"triggers": []map[string]interface{}{
					{
						"type": "cron",
						metadataKey: map[string]interface{}{
							"timezone":        cfg.KedaCronTimezone,
							"start":           cfg.KedaCronStart,
							"end":             cfg.KedaCronEnd,
							"desiredReplicas": desiredReplicas,
						},
					},
				},
			},
		},
	}
	return scaledObject
}
