// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package azapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/azure/azure-dev/cli/azd/pkg/azure"
)

type Deployments interface {
	ListSubscriptionDeployments(
		ctx context.Context,
	) ([]*armresources.DeploymentExtended, error)
	GetSubscriptionDeployment(
		ctx context.Context,
		deploymentName string,
	) (*armresources.DeploymentExtended, error)
	ListResourceGroupDeployments(
		ctx context.Context,
		resourceGroupName string,
	) ([]*armresources.DeploymentExtended, error)
	GetResourceGroupDeployment(
		ctx context.Context,
		resourceGroupName string,
		deploymentName string,
	) (*armresources.DeploymentExtended, error)
	DeployToSubscription(
		ctx context.Context,
		location string,
		deploymentName string,
		armTemplate azure.RawArmTemplate,
		parameters azure.ArmParameters,
		tags map[string]*string,
	) (*armresources.DeploymentExtended, error)
	DeployToResourceGroup(
		ctx context.Context,
		resourceGroup,
		deploymentName string,
		armTemplate azure.RawArmTemplate,
		parameters azure.ArmParameters,
		tags map[string]*string,
	) (*armresources.DeploymentExtended, error)
	WhatIfDeployToSubscription(
		ctx context.Context,
		location string,
		deploymentName string,
		armTemplate azure.RawArmTemplate,
		parameters azure.ArmParameters,
	) (*armresources.WhatIfOperationResult, error)
	WhatIfDeployToResourceGroup(
		ctx context.Context,
		resourceGroup,
		deploymentName string,
		armTemplate azure.RawArmTemplate,
		parameters azure.ArmParameters,
	) (*armresources.WhatIfOperationResult, error)
	DeleteSubscriptionDeployment(ctx context.Context, deploymentName string) error
	CalculateTemplateHash(
		ctx context.Context,
		template azure.RawArmTemplate,
	) (armresources.DeploymentsClientCalculateTemplateHashResponse, error)
}

var (
	ErrDeploymentNotFound = errors.New("deployment not found")
)

type deployments struct {
	deploymentsClient *armresources.DeploymentsClient
}

func NewDeployments(
	deploymentsClient *armresources.DeploymentsClient,
) Deployments {
	return &deployments{
		deploymentsClient: deploymentsClient,
	}
}

func (ds *deployments) CalculateTemplateHash(
	ctx context.Context,
	template azure.RawArmTemplate,
) (result armresources.DeploymentsClientCalculateTemplateHashResponse, err error) {
	return ds.deploymentsClient.CalculateTemplateHash(ctx, template, nil)
}

func (ds *deployments) ListSubscriptionDeployments(
	ctx context.Context,
) ([]*armresources.DeploymentExtended, error) {

	results := []*armresources.DeploymentExtended{}

	pager := ds.deploymentsClient.NewListAtSubscriptionScopePager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		results = append(results, page.Value...)
	}

	return results, nil
}

func (ds *deployments) GetSubscriptionDeployment(
	ctx context.Context,
	deploymentName string,
) (*armresources.DeploymentExtended, error) {

	deployment, err := ds.deploymentsClient.GetAtSubscriptionScope(ctx, deploymentName, nil)
	if err != nil {
		var errDetails *azcore.ResponseError
		if errors.As(err, &errDetails) && errDetails.StatusCode == 404 {
			return nil, ErrDeploymentNotFound
		}
		return nil, fmt.Errorf("getting deployment from subscription: %w", err)
	}

	return &deployment.DeploymentExtended, nil
}

func (ds *deployments) ListResourceGroupDeployments(
	ctx context.Context,
	resourceGroupName string,
) ([]*armresources.DeploymentExtended, error) {

	results := []*armresources.DeploymentExtended{}

	pager := ds.deploymentsClient.NewListByResourceGroupPager(resourceGroupName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		results = append(results, page.Value...)
	}

	return results, nil
}

func (ds *deployments) GetResourceGroupDeployment(
	ctx context.Context,
	resourceGroupName string,
	deploymentName string,
) (*armresources.DeploymentExtended, error) {

	deployment, err := ds.deploymentsClient.Get(ctx, resourceGroupName, deploymentName, nil)
	if err != nil {
		var errDetails *azcore.ResponseError
		if errors.As(err, &errDetails) && errDetails.StatusCode == 404 {
			return nil, ErrDeploymentNotFound
		}
		return nil, fmt.Errorf("getting deployment from resource group: %w", err)
	}

	return &deployment.DeploymentExtended, nil
}

func (ds *deployments) DeployToSubscription(
	ctx context.Context,
	location string,
	deploymentName string,
	armTemplate azure.RawArmTemplate,
	parameters azure.ArmParameters,
	tags map[string]*string,
) (*armresources.DeploymentExtended, error) {
	createFromTemplateOperation, err := ds.deploymentsClient.BeginCreateOrUpdateAtSubscriptionScope(
		ctx, deploymentName,
		armresources.Deployment{
			Properties: &armresources.DeploymentProperties{
				Template:   armTemplate,
				Parameters: parameters,
				Mode:       to.Ptr(armresources.DeploymentModeIncremental),
			},
			Location: to.Ptr(location),
			Tags:     tags,
		}, nil)
	if err != nil {
		return nil, fmt.Errorf("starting deployment to subscription: %w", err)
	}

	// wait for deployment creation
	deployResult, err := createFromTemplateOperation.PollUntilDone(ctx, nil)
	if err != nil {
		deploymentError := createDeploymentError(err)
		return nil, fmt.Errorf(
			"deploying to subscription:\n\nDeployment Error Details:\n%w",
			deploymentError,
		)
	}

	return &deployResult.DeploymentExtended, nil
}

func (ds *deployments) DeployToResourceGroup(
	ctx context.Context,
	resourceGroup, deploymentName string,
	armTemplate azure.RawArmTemplate,
	parameters azure.ArmParameters,
	tags map[string]*string,
) (*armresources.DeploymentExtended, error) {
	createFromTemplateOperation, err := ds.deploymentsClient.BeginCreateOrUpdate(
		ctx, resourceGroup, deploymentName,
		armresources.Deployment{
			Properties: &armresources.DeploymentProperties{
				Template:   armTemplate,
				Parameters: parameters,
				Mode:       to.Ptr(armresources.DeploymentModeIncremental),
			},
			Tags: tags,
		}, nil)
	if err != nil {
		return nil, fmt.Errorf("starting deployment to resource group: %w", err)
	}

	// wait for deployment creation
	deployResult, err := createFromTemplateOperation.PollUntilDone(ctx, nil)
	if err != nil {
		deploymentError := createDeploymentError(err)
		return nil, fmt.Errorf(
			"deploying to resource group:\n\nDeployment Error Details:\n%w",
			deploymentError,
		)
	}

	return &deployResult.DeploymentExtended, nil
}

func (ds *deployments) WhatIfDeployToSubscription(
	ctx context.Context,
	location string,
	deploymentName string,
	armTemplate azure.RawArmTemplate,
	parameters azure.ArmParameters,
) (*armresources.WhatIfOperationResult, error) {
	createFromTemplateOperation, err := ds.deploymentsClient.BeginWhatIfAtSubscriptionScope(
		ctx, deploymentName,
		armresources.DeploymentWhatIf{
			Properties: &armresources.DeploymentWhatIfProperties{
				Template:       armTemplate,
				Parameters:     parameters,
				Mode:           to.Ptr(armresources.DeploymentModeIncremental),
				WhatIfSettings: &armresources.DeploymentWhatIfSettings{},
			},
			Location: to.Ptr(location),
		}, nil)
	if err != nil {
		return nil, fmt.Errorf("starting deployment to subscription: %w", err)
	}

	// wait for deployment creation
	deployResult, err := createFromTemplateOperation.PollUntilDone(ctx, nil)
	if err != nil {
		deploymentError := createDeploymentError(err)
		return nil, fmt.Errorf(
			"deploying to subscription:\n\nDeployment Error Details:\n%w",
			deploymentError,
		)
	}

	return &deployResult.WhatIfOperationResult, nil
}

func (ds *deployments) WhatIfDeployToResourceGroup(
	ctx context.Context,
	resourceGroup, deploymentName string,
	armTemplate azure.RawArmTemplate,
	parameters azure.ArmParameters,
) (*armresources.WhatIfOperationResult, error) {
	createFromTemplateOperation, err := ds.deploymentsClient.BeginWhatIf(
		ctx, resourceGroup, deploymentName,
		armresources.DeploymentWhatIf{
			Properties: &armresources.DeploymentWhatIfProperties{
				Template:   armTemplate,
				Parameters: parameters,
				Mode:       to.Ptr(armresources.DeploymentModeIncremental),
			},
		}, nil)
	if err != nil {
		return nil, fmt.Errorf("starting deployment to resource group: %w", err)
	}

	// wait for deployment creation
	deployResult, err := createFromTemplateOperation.PollUntilDone(ctx, nil)
	if err != nil {
		deploymentError := createDeploymentError(err)
		return nil, fmt.Errorf(
			"deploying to resource group:\n\nDeployment Error Details:\n%w",
			deploymentError,
		)
	}

	return &deployResult.WhatIfOperationResult, nil
}

func (ds *deployments) DeleteSubscriptionDeployment(
	ctx context.Context,
	deploymentName string,
) error {
	deleteDeploymentOperation, err := ds.deploymentsClient.BeginDeleteAtSubscriptionScope(ctx, deploymentName, nil)
	if err != nil {
		return fmt.Errorf("starting to delete deployment: %w", err)
	}

	// wait for the operation to complete
	_, err = deleteDeploymentOperation.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("deleting deployment operation: %w", err)
	}

	return nil
}

type AzCliDeploymentPropertiesDependency struct {
	AzCliDeploymentPropertiesBasicDependency
	DependsOn []AzCliDeploymentPropertiesBasicDependency `json:"dependsOn"`
}

type AzCliDeploymentPropertiesBasicDependency struct {
	Id           string `json:"id"`
	ResourceName string `json:"resourceName"`
	ResourceType string `json:"resourceType"`
}

type AzCliDeploymentErrorResponse struct {
	Code           string                         `json:"code"`
	Message        string                         `json:"message"`
	Target         string                         `json:"target"`
	Details        []AzCliDeploymentErrorResponse `json:"details"`
	AdditionalInfo AzCliDeploymentAdditionalInfo  `json:"additionalInfo"`
}

type AzCliDeploymentAdditionalInfo struct {
	Type string      `json:"type"`
	Info interface{} `json:"info"`
}

type AzCliDeployment struct {
	Id         string                    `json:"id"`
	Name       string                    `json:"name"`
	Properties AzCliDeploymentProperties `json:"properties"`
}

type AzCliDeploymentProperties struct {
	CorrelationId   string                                `json:"correlationId"`
	Error           AzCliDeploymentErrorResponse          `json:"error"`
	Dependencies    []AzCliDeploymentPropertiesDependency `json:"dependencies"`
	OutputResources []AzCliDeploymentResourceReference    `json:"outputResources"`
	Outputs         map[string]AzCliDeploymentOutput      `json:"outputs"`
}

type AzCliDeploymentResourceReference struct {
	Id string `json:"id"`
}

type AzCliDeploymentOutput struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

type AzCliDeploymentResult struct {
	Properties AzCliDeploymentResultProperties `json:"properties"`
}

type AzCliDeploymentResultProperties struct {
	Outputs map[string]AzCliDeploymentOutput `json:"outputs"`
}

type AzCliResourceOperation struct {
	Id          string                           `json:"id"`
	OperationId string                           `json:"operationId"`
	Properties  AzCliResourceOperationProperties `json:"properties"`
}

type AzCliResourceOperationProperties struct {
	ProvisioningOperation string                               `json:"provisioningOperation"`
	ProvisioningState     string                               `json:"provisioningState"`
	TargetResource        AzCliResourceOperationTargetResource `json:"targetResource"`
	StatusCode            string                               `json:"statusCode"`
	StatusMessage         AzCliDeploymentStatusMessage         `json:"statusMessage"`
	// While the operation is in progress, this timestamp effectively represents "InProgressTimestamp".
	// When the operation ends, this timestamp effectively represents "EndTimestamp".
	Timestamp time.Time `json:"timestamp"`
}

type AzCliResourceOperationTargetResource struct {
	Id            string `json:"id"`
	ResourceType  string `json:"resourceType"`
	ResourceName  string `json:"resourceName"`
	ResourceGroup string `json:"resourceGroup"`
}

type AzCliDeploymentStatusMessage struct {
	Err    AzCliDeploymentErrorResponse `json:"error"`
	Status string                       `json:"status"`
}

// convert from: sdk client outputs: interface{} to map[string]azapi.AzCliDeploymentOutput
// sdk client parses http response from network as an interface{}
// this function keeps the compatibility with the previous AzCliDeploymentOutput model
func CreateDeploymentOutput(rawOutputs interface{}) (result map[string]AzCliDeploymentOutput) {
	if rawOutputs == nil {
		return make(map[string]AzCliDeploymentOutput, 0)
	}

	castInput := rawOutputs.(map[string]interface{})
	result = make(map[string]AzCliDeploymentOutput, len(castInput))
	for key, output := range castInput {
		innerValue := output.(map[string]interface{})
		result[key] = AzCliDeploymentOutput{
			Type:  innerValue["type"].(string),
			Value: innerValue["value"],
		}
	}
	return result
}

// Attempts to create an Azure Deployment error from the HTTP response error
func createDeploymentError(err error) error {
	var responseErr *azcore.ResponseError
	if errors.As(err, &responseErr) {
		var errorText string
		rawBody, err := io.ReadAll(responseErr.RawResponse.Body)
		if err != nil {
			errorText = responseErr.Error()
		} else {
			errorText = string(rawBody)
		}
		return NewAzureDeploymentError(errorText)
	}

	return err
}
