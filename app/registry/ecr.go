package registry

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/gold-kou/prism-in-k8s/app/params"
)

func BuildAndPushECR(ctx context.Context) error {
	// build Docker image
	imageTag := params.MicroserviceName + ":latest"
	cmd := exec.Command("docker", "build", "-f", "Dockerfile.prism", "-t", imageTag, ".")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build docker image: %v", err)
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
			return fmt.Errorf("failed to create ECR repository: %v", err)
		}
		log.Println("[WARN] The ECR already exists")
	} else {
		log.Println("[INFO] ECR is created successfully")
	}

	// tag Docker image for ECR
	ecrImageTag := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:latest", params.AWSAccountID, params.AWSConfig.Region, repositoryName)
	cmdTag := exec.Command("docker", "tag", imageTag, ecrImageTag)
	if err := cmdTag.Run(); err != nil {
		return fmt.Errorf("failed to tag image: %v", err)
	}
	log.Println("[INFO] Docker image tagged successfully")

	// login to ECR
	loginCommand := fmt.Sprintf("aws ecr get-login-password --region %s | docker login --username AWS --password-stdin %s.dkr.ecr.%s.amazonaws.com", params.AWSConfig.Region, params.AWSAccountID, params.AWSConfig.Region)
	cmdLogin := exec.Command("bash", "-c", loginCommand)
	if err := cmdLogin.Run(); err != nil {
		return fmt.Errorf("failed to log in ECR: %v", err)
	}
	log.Println("[INFO] Logged in ECR successfully")

	// push image to ECR
	cmdPush := exec.Command("docker", "push", ecrImageTag)
	if err := cmdPush.Run(); err != nil {
		return fmt.Errorf("failed to push image to ECR: %v", err)
	}
	log.Println("[INFO] Docker image is pushed to ECR successfully")
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
			return fmt.Errorf("failed to delete ECR: %v", err)
		}
		log.Println("[WARN] The ECR is not found")
	} else {
		log.Println("[INFO] ECR is deleted successfully")
	}
	return nil
}
