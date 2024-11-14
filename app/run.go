package app

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gold-kou/prism-in-k8s/app/istio"
	"github.com/gold-kou/prism-in-k8s/app/k8s"
	"github.com/gold-kou/prism-in-k8s/app/params"
	"github.com/gold-kou/prism-in-k8s/app/registry"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	isCreate   bool
	isDelete   bool
	kubeConfig *restclient.Config
)

func init() {
	// command args
	flag.BoolVar(&isCreate, "create", false, "set to true if running in create mode")
	flag.BoolVar(&isDelete, "delete", false, "set to true if running in delete mode")
	flag.BoolVar(&params.IsTest, "test", false, "set to true if running in test mode")
	flag.Parse()

	// empty check for openapi.yaml
	data, err := os.ReadFile("openapi.yaml")
	if err != nil {
		panic(err)
	}
	if len(data) == 0 {
		panic("openapi.yaml is empty")
	}

	// validation parameters
	err = params.ValidateParams()
	if err != nil {
		panic(err)
	}

	// AWS config
	params.AWSConfig, err = config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(fmt.Errorf("failed load AWS config: %v", err))
	}

	// get AWS account ID
	stsClient := sts.NewFromConfig(params.AWSConfig)
	result, err := stsClient.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
	if err != nil {
		panic(fmt.Errorf("failed to get caller identity: %v", err))
	}
	params.AWSAccountID = *result.Account

	// kube config
	kubeconfigPath := clientcmd.NewDefaultPathOptions().GetDefaultFilename()
	kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		panic(fmt.Errorf("failed to build Kubeconfig: %v", err))
	}
}

func Run() {
	ctx, cancel := context.WithTimeout(context.Background(), params.Timeout)
	defer cancel()

	if isCreate {
		err := registry.BuildAndPushECR(ctx)
		if err != nil {
			panic(err)
		}

		err = k8s.CreateK8sResources(ctx, kubeConfig, params.ResourceName, params.NamespaceName)
		if err != nil {
			panic(err)
		}

		err = istio.CreateIstioResources(ctx, kubeConfig, params.ResourceName, params.NamespaceName)
		if err != nil {
			panic(err)
		}
		log.Println("[INFO] All resources for prism mock are created successfully")
	} else if isDelete {
		err := istio.DeleteIstioResources(ctx, kubeConfig, params.ResourceName, params.NamespaceName)
		if err != nil {
			panic(err)
		}

		err = k8s.DeleteK8sResources(ctx, kubeConfig, params.ResourceName, params.NamespaceName)
		if err != nil {
			panic(err)
		}

		err = registry.DeleteECR(ctx)
		if err != nil {
			panic(err)
		}
		log.Println("[INFO] All resources for prism mock are deleted successfully")
	}
}
