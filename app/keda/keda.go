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

	scaledObject := BuildScaledObject(cfg, resourceName)

	_, err = dynamicClient.Resource(scaledObjectGVR).Namespace(namespaceName).Create(ctx, scaledObject, metav1.CreateOptions{})
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create ScaledObject: %w", err)
		}
		slog.Warn("The ScaledObject already exists")
	} else {
		slog.Info("ScaledObject is created successfully")
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
			return fmt.Errorf("failed to delete ScaledObject: %w", err)
		}
		slog.Warn("The ScaledObject is not found")
	} else {
		slog.Info("ScaledObject is deleted successfully")
	}
	return nil
}

func BuildScaledObject(cfg *params.Config, resourceName string) *unstructured.Unstructured {
	maxReplicas := int64(1)
	if v, err := strconv.ParseInt(cfg.KedaDesiredReplicas, 10, 64); err == nil && v > maxReplicas {
		maxReplicas = v
	}

	scaledObject := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "keda.sh/v1alpha1",
			"kind":       "ScaledObject",
			"metadata": map[string]interface{}{
				"name": resourceName,
			},
			"spec": map[string]interface{}{
				"scaleTargetRef": map[string]interface{}{
					"name": resourceName,
				},
				"minReplicaCount": int64(0),
				"maxReplicaCount": maxReplicas,
				"triggers": []map[string]interface{}{
					{
						"type": "cron",
						"metadata": map[string]interface{}{
							"timezone":        cfg.KedaCronTimezone,
							"start":           cfg.KedaCronStart,
							"end":             cfg.KedaCronEnd,
							"desiredReplicas": cfg.KedaDesiredReplicas,
						},
					},
				},
			},
		},
	}
	return scaledObject
}
