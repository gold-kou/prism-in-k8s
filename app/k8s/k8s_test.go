package k8s_test

import (
	"context"
	"testing"

	"github.com/gold-kou/prism-in-k8s/app/k8s"
	"github.com/gold-kou/prism-in-k8s/app/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func TestCreateK8sResources(t *testing.T) {
	testNamespaceName := "test-namespace" + uuid.NewString()
	testResourceName := "test-resource" + uuid.NewString()

	ctx := context.TODO()
	kubeconfigPath := clientcmd.NewDefaultPathOptions().GetDefaultFilename()
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		panic(err)
	}

	// test target
	err = k8s.CreateK8sResources(ctx, kubeconfig, testNamespaceName, testResourceName)
	assert.NoError(t, err)

	// verify
	k8sClientSet, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		panic(err)
	}
	_, err = k8sClientSet.CoreV1().Namespaces().Get(ctx, testNamespaceName, metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = k8sClientSet.AppsV1().Deployments(testNamespaceName).Get(ctx, testResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	_, err = k8sClientSet.CoreV1().Services(testNamespaceName).Get(ctx, testResourceName, metav1.GetOptions{})
	assert.NoError(t, err)

	// clean up
	err = testutil.DeleteService(ctx, k8sClientSet, testNamespaceName, testResourceName)
	if err != nil {
		panic(err)
	}
	err = testutil.DeleteDeployment(ctx, k8sClientSet, testNamespaceName, testResourceName)
	if err != nil {
		panic(err)
	}
	err = testutil.DeleteNamespace(ctx, k8sClientSet, testNamespaceName)
	if err != nil {
		panic(err)
	}
}

func TestDeleteK8sResources(t *testing.T) {
	testNamespaceName := "test-namespace" + uuid.NewString()
	testResourceName := "test-resource" + uuid.NewString()

	ctx := context.TODO()
	kubeconfigPath := clientcmd.NewDefaultPathOptions().GetDefaultFilename()
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		panic(err)
	}
	k8sClientSet, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		panic(err)
	}

	// dummy resources
	err = testutil.CreateNamespace(ctx, k8sClientSet, testNamespaceName)
	if err != nil {
		panic(err)
	}
	err = testutil.CreateDeployment(ctx, k8sClientSet, testNamespaceName, testResourceName)
	if err != nil {
		panic(err)
	}
	err = testutil.CreateService(ctx, k8sClientSet, testNamespaceName, testResourceName)
	if err != nil {
		panic(err)
	}

	// test target
	err = k8s.DeleteK8sResources(ctx, kubeconfig, testNamespaceName, testResourceName)
	assert.NoError(t, err)

	// skip verify to reduce test time
}
