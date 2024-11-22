package registry

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/gold-kou/prism-in-k8s/app/params"
	"golang.org/x/xerrors"
)

var (
	errFailedToBuildDockerImage = errors.New("failed to build docker image")
	errFailedToCreateECR        = errors.New("failed to create ECR repository")
	errFailedToTagImage         = errors.New("failed to tag image")
	errFailedToLoginECR         = errors.New("failed to log in ECR")
	errFailedToPushImage        = errors.New("failed to push image to ECR")
	errFailedToDeleteECR        = errors.New("failed to delete ECR repository")
)

func BuildAndPushECR(ctx context.Context) error {
	// build Docker image
	imageTag := params.MicroserviceName + ":latest"
	cmd := exec.Command("docker", "build", "-f", "Dockerfile.prism", "-t", imageTag, ".")
	if err := cmd.Run(); err != nil {
		return xerrors.Errorf("%s: %v", errFailedToBuildDockerImage, err)
	}
	log.Println("[INFO] Docker image is built successfully")

	// create ECR repository
	ecrClient := ecr.NewFromConfig(params.AWSConfig)
	repositoryName := params.ResourceName
	input := &ecr.CreateRepositoryInput{
		RepositoryName: aws.String(repositoryName),
		Tags: []types.Tag{
			{
				Key:   aws.String("CostEnv"),
				Value: aws.String(params.EcrTagEnv),
			},
			{
				Key:   aws.String("CostService"),
				Value: aws.String(params.MicroserviceName),
			},
		},
	}
	_, err := ecrClient.CreateRepository(ctx, input)
	if err != nil {
		var ecrExistsException *types.RepositoryAlreadyExistsException
		if !errors.As(err, &ecrExistsException) {
			return xerrors.Errorf("%w: %w", errFailedToCreateECR, err)
		}
		log.Println("[WARN] The ECR already exists")
	} else {
		log.Println("[INFO] ECR is created successfully")
	}

	// tag Docker image for ECR
	ecrImageTag := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:latest", params.AWSAccountID, params.AWSConfig.Region, repositoryName)
	cmdTag := exec.Command("docker", "tag", imageTag, ecrImageTag)
	if err := cmdTag.Run(); err != nil {
		return xerrors.Errorf("%w: %w", errFailedToTagImage, err)
	}
	log.Println("[INFO] Docker image tagged successfully")

	// login to ECR
	err = loginToECR(ctx, params.AWSConfig, params.AWSAccountID)
	if err != nil {
		return xerrors.Errorf("%w: %w", errFailedToLoginECR, err)
	}
	log.Println("[INFO] Logged in ECR successfully")

	// push image to ECR
	cmdPush := exec.Command("docker", "push", ecrImageTag)
	if err := cmdPush.Run(); err != nil {
		return xerrors.Errorf("%w: %w", errFailedToPushImage, err)
	}
	log.Println("[INFO] Docker image is pushed to ECR successfully")
	return nil
}

func loginToECR(ctx context.Context, awsConfig aws.Config, awsAccountID string) error {
	ecrClient := ecr.NewFromConfig(awsConfig)

	// Get the authorization token
	authTokenOutput, err := ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{
		RegistryIds: []string{awsAccountID},
	})
	if err != nil {
		return fmt.Errorf("%w: %w", errFailedToLoginECR, err)
	}

	if len(authTokenOutput.AuthorizationData) == 0 {
		return fmt.Errorf("%w: no authorization data found", errFailedToLoginECR)
	}

	authData := authTokenOutput.AuthorizationData[0]
	decodedToken, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
	if err != nil {
		return fmt.Errorf("%w: %w", errFailedToLoginECR, err)
	}

	decodedTokenParts := 2
	parts := strings.SplitN(string(decodedToken), ":", decodedTokenParts)
	if len(parts) != decodedTokenParts {
		return fmt.Errorf("%w: invalid authorization token format", errFailedToLoginECR)
	}

	username := parts[0]
	password := parts[1]
	registry := *authData.ProxyEndpoint

	loginCmd := exec.Command("docker", "login", "--username", username, "--password-stdin", registry)
	loginCmd.Stdin = strings.NewReader(password)
	output, err := loginCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %w\n%s", errFailedToLoginECR, err, string(output))
	}

	return nil
}

func DeleteECR(ctx context.Context) error {
	// Delete ECR
	ecrClient := ecr.NewFromConfig(params.AWSConfig)
	repositoryName := params.ResourceName
	input := &ecr.DeleteRepositoryInput{
		RepositoryName: aws.String(repositoryName),
		Force:          true, // Force delete to remove all images
	}
	_, err := ecrClient.DeleteRepository(ctx, input)
	if err != nil {
		var ecrNotFoundException *types.RepositoryNotFoundException
		if !errors.As(err, &ecrNotFoundException) {
			return xerrors.Errorf("%w: %w", errFailedToDeleteECR, err)
		}
		log.Println("[WARN] The ECR is not found")
	} else {
		log.Println("[INFO] ECR is deleted successfully")
	}
	return nil
}
