package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	action        actionType
	awsConfig     aws.Config
	awsAccountID  string
	kubeConfig    *restclient.Config
	resourceName  string
	namespaceName string
)

func init() {
	//empty check for openapi.yaml
	data, err := os.ReadFile("openapi.yaml")
	if err != nil {
		panic(err)
	}
	if len(data) == 0 {
		panic("openapi.yaml is empty")
	}

	// validation parameters
	err = validateParams()
	if err != nil {
		panic(err)
	}

	// action parameter
	var actionStr string
	flag.StringVar(&actionStr, "action", "create", "create or delete(default: create)")
	flag.Parse()
	parsedAction, err := validateActionType(actionStr)
	if err != nil {
		panic(err)
	}
	action = parsedAction

	// AWS config
	awsConfig, err = config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(fmt.Errorf("failed load AWS config: %v", err))
	}

	// get AWS account ID
	stsClient := sts.NewFromConfig(awsConfig)
	result, err := stsClient.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
	if err != nil {
		panic(fmt.Errorf("failed to get caller identity: %v", err))
	}
	awsAccountID = *result.Account

	// kube config
	kubeconfigPath := clientcmd.NewDefaultPathOptions().GetDefaultFilename()
	kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		panic(fmt.Errorf("failed to build kubeconfig: %v", err))
	}

	// resource name
	resourceName = microserviceName + prismMockSuffix
	namespaceName = microserviceNamespace + prismMockSuffix
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if action == create {
		err := buildAndPushECR(ctx)
		if err != nil {
			panic(err)
		}
		err = createK8sResources(ctx)
		if err != nil {
			panic(err)
		}
		err = createIstioResources(ctx)
		if err != nil {
			panic(err)
		}
		log.Println("[INFO] All resources for prism mock are created successfully")
	} else if action == delete {
		err := deleteIstioResources(ctx)
		if err != nil {
			panic(err)
		}
		err = deleteK8sResources(ctx)
		if err != nil {
			panic(err)
		}
		err = deleteECR(ctx)
		if err != nil {
			panic(err)
		}
		log.Println("[INFO] All resources for prism mock are deleted successfully")
	}
}
