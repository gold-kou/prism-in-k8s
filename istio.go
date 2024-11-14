package main

import (
	"context"
	"fmt"
	"log"

	"github.com/golang/protobuf/ptypes/duration"
	"github.com/pingcap/errors"
	networkingv1alpha3 "istio.io/api/networking/v1alpha3"
	"istio.io/client-go/pkg/apis/networking/v1alpha3"
	"istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createIstioResources(ctx context.Context) error {
	// Istio clientset
	istioClient, err := versioned.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create Istio client: %v", err)
	}

	// VirtualService
	virtualService := &v1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name: resourceName,
		},
		Spec: networkingv1alpha3.VirtualService{
			Hosts: []string{
				resourceName + "." + namespaceName + ".svc.cluster.local",
			},
			Http: []*networkingv1alpha3.HTTPRoute{
				{
					Name: "example1",
					Match: []*networkingv1alpha3.HTTPMatchRequest{
						{
							Uri: &networkingv1alpha3.StringMatch{
								MatchType: &networkingv1alpha3.StringMatch_Prefix{
									Prefix: "/example1/",
								},
							},
							Method: &networkingv1alpha3.StringMatch{
								MatchType: &networkingv1alpha3.StringMatch_Exact{
									Exact: "GET",
								},
							},
						},
					},
					Fault: &networkingv1alpha3.HTTPFaultInjection{
						Delay: &networkingv1alpha3.HTTPFaultInjection_Delay{
							Percentage: &networkingv1alpha3.Percent{
								Value: 100.0,
							},
							HttpDelayType: &networkingv1alpha3.HTTPFaultInjection_Delay_FixedDelay{
								FixedDelay: &duration.Duration{Nanos: int32(100000000)}, // 100ms
							},
						},
					},
					Route: []*networkingv1alpha3.HTTPRouteDestination{
						{
							Destination: &networkingv1alpha3.Destination{
								Host: resourceName + "." + namespaceName + ".svc.cluster.local",
							},
						},
					},
				},
				{
					Name: "default",
					Route: []*networkingv1alpha3.HTTPRouteDestination{
						{
							Destination: &networkingv1alpha3.Destination{
								Host: resourceName + "." + namespaceName + ".svc.cluster.local",
							},
						},
					},
				},
			},
		},
	}
	_, err = istioClient.NetworkingV1alpha3().VirtualServices(namespaceName).Create(ctx, virtualService, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create VirtualService: %v", err)
		}
		log.Println("[WARN] The VirtualService already exists")
	} else {
		log.Println("[INFO] VirtualService is created successfully")
	}
	return nil
}

func deleteIstioResources(ctx context.Context) error {
	// Istio clientset
	istioClient, err := versioned.NewForConfig(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to create Istio client: %v", err)
	}
	log.Println("[INFO] Clientset of istio set up successfully")

	err = istioClient.NetworkingV1alpha3().VirtualServices(namespaceName).Delete(ctx, resourceName, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to create VirtualService: %v", err)
		}
		log.Println("[WARN] The VirtualService is not found")
	} else {
		log.Println("[INFO] VirtualService is deleted successfully")
	}
	return nil
}
