package istio_test

import (
	"context"
	"testing"

	"github.com/gold-kou/prism-in-k8s/app/istio"
	"github.com/gold-kou/prism-in-k8s/app/params"
	"github.com/gold-kou/prism-in-k8s/app/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"istio.io/client-go/pkg/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func TestCreateIstioResources(t *testing.T) {
	testNamespaceName := "test-namespace" + uuid.NewString()
	testResourceName := "test-resource" + uuid.NewString()

	ctx := context.TODO()
	kubeconfigPath := clientcmd.NewDefaultPathOptions().GetDefaultFilename()
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	require.NoError(t, err)

	// create namespace to create a virtualservice
	k8sClientSet, err := kubernetes.NewForConfig(kubeconfig)
	require.NoError(t, err)
	err = testutil.CreateNamespace(ctx, k8sClientSet, testNamespaceName)
	require.NoError(t, err)

	// test target
	routes := []params.VirtualServiceRoute{
		{
			Name:            "example1",
			URIPrefix:       "/example1/",
			Method:          "GET",
			DelayNanos:      100000000,
			DelayPercentage: 100,
		},
		{
			Name:      "example2",
			URIPrefix: "/example2/",
			Method:    "POST",
		},
	}
	err = istio.CreateIstioResources(ctx, kubeconfig, testNamespaceName, testResourceName, routes)
	assert.NoError(t, err)

	// verify
	istioClient, err := versioned.NewForConfig(kubeconfig)
	assert.NoError(t, err)
	vs, err := istioClient.NetworkingV1alpha3().VirtualServices(testNamespaceName).Get(ctx, testResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	// verify configured routes + trailing default catch-all
	require.Len(t, vs.Spec.Http, 3)
	assert.Equal(t, "example1", vs.Spec.Http[0].Name)
	assert.Equal(t, "/example1/", vs.Spec.Http[0].Match[0].Uri.GetPrefix())
	assert.Equal(t, "GET", vs.Spec.Http[0].Match[0].Method.GetExact())
	assert.Equal(t, int32(100000000), vs.Spec.Http[0].Fault.Delay.GetFixedDelay().Nanos)
	assert.InDelta(t, 100.0, vs.Spec.Http[0].Fault.Delay.Percentage.Value, 0.001)
	assert.Equal(t, "example2", vs.Spec.Http[1].Name)
	assert.Equal(t, "/example2/", vs.Spec.Http[1].Match[0].Uri.GetPrefix())
	assert.Equal(t, "POST", vs.Spec.Http[1].Match[0].Method.GetExact())
	assert.Nil(t, vs.Spec.Http[1].Fault)
	assert.Equal(t, "default", vs.Spec.Http[2].Name)

	// clean up
	err = testutil.DeleteNamespace(ctx, k8sClientSet, testNamespaceName)
	require.NoError(t, err)
}

func TestDeleteIstioResources(t *testing.T) {
	testNamespaceName := "test-namespace" + uuid.NewString()
	testResourceName := "test-resource" + uuid.NewString()

	ctx := context.TODO()
	kubeconfigPath := clientcmd.NewDefaultPathOptions().GetDefaultFilename()
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	require.NoError(t, err)
	k8sClientSet, err := kubernetes.NewForConfig(kubeconfig)
	require.NoError(t, err)
	istioClientSet, err := versioned.NewForConfig(kubeconfig)
	require.NoError(t, err)

	// dummy resources
	err = testutil.CreateNamespace(ctx, k8sClientSet, testNamespaceName)
	require.NoError(t, err)
	err = testutil.CreateVirtualService(ctx, istioClientSet, testNamespaceName, testResourceName)
	require.NoError(t, err)

	// test target
	err = istio.DeleteIstioResources(ctx, kubeconfig, testNamespaceName, testResourceName)
	assert.NoError(t, err)

	// verify
	_, err = istioClientSet.NetworkingV1alpha3().VirtualServices(testNamespaceName).Get(ctx, testResourceName, metav1.GetOptions{})
	assert.Error(t, err)
	assert.True(t, apierrors.IsNotFound(err))

	// clean up
	err = testutil.DeleteNamespace(ctx, k8sClientSet, testNamespaceName)
	require.NoError(t, err)
}
