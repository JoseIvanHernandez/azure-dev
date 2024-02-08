package mocks

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/azure/azure-dev/cli/azd/pkg/alpha"
	"github.com/azure/azure-dev/cli/azd/pkg/config"
	"github.com/azure/azure-dev/cli/azd/pkg/exec"
	"github.com/azure/azure-dev/cli/azd/pkg/httputil"
	"github.com/azure/azure-dev/cli/azd/pkg/input"
	"github.com/azure/azure-dev/cli/azd/pkg/ioc"
	"github.com/azure/azure-dev/cli/azd/test/mocks/mockconfig"
	"github.com/azure/azure-dev/cli/azd/test/mocks/mockexec"
	"github.com/azure/azure-dev/cli/azd/test/mocks/mockhttp"
	"github.com/azure/azure-dev/cli/azd/test/mocks/mockinput"
)

type MockContext struct {
	Credentials                    *MockCredentials
	Context                        *context.Context
	Console                        *mockinput.MockConsole
	HttpClient                     *mockhttp.MockHttpClient
	CoreClientOptions              *policy.ClientOptions
	ArmClientOptions               *arm.ClientOptions
	CommandRunner                  *mockexec.MockCommandRunner
	ConfigManager                  *mockconfig.MockConfigManager
	Container                      *ioc.NestedContainer
	AlphaFeaturesManager           *alpha.FeatureManager
	SubscriptionCredentialProvider *MockSubscriptionCredentialProvider
	MultiTenantCredentialProvider  *MockMultiTenantCredentialProvider
	Config                         config.Config
}

func NewMockContext(ctx context.Context) *MockContext {
	httpClient := mockhttp.NewMockHttpUtil()
	configManager := mockconfig.NewMockConfigManager()
	config := config.NewEmptyConfig()

	coreClientOptions := &azcore.ClientOptions{
		Transport: httpClient,
	}
	armClientOptions := &arm.ClientOptions{
		ClientOptions: policy.ClientOptions{
			Transport: httpClient,
		},
	}

	mockContext := &MockContext{
		Credentials:                    &MockCredentials{},
		Context:                        &ctx,
		Console:                        mockinput.NewMockConsole(),
		CommandRunner:                  mockexec.NewMockCommandRunner(),
		HttpClient:                     httpClient,
		CoreClientOptions:              coreClientOptions,
		ArmClientOptions:               armClientOptions,
		ConfigManager:                  configManager,
		SubscriptionCredentialProvider: &MockSubscriptionCredentialProvider{},
		MultiTenantCredentialProvider:  &MockMultiTenantCredentialProvider{},
		Container:                      ioc.NewNestedContainer(nil),
		Config:                         config,
		AlphaFeaturesManager:           alpha.NewFeaturesManagerWithConfig(config),
	}

	registerCommonMocks(mockContext)

	return mockContext
}

func registerCommonMocks(mockContext *MockContext) {
	mockContext.Container.MustRegisterSingleton(func() ioc.ServiceLocator {
		return mockContext.Container
	})
	mockContext.Container.MustRegisterSingleton(func() azcore.TokenCredential {
		return mockContext.Credentials
	})
	mockContext.Container.MustRegisterSingleton(func() httputil.HttpClient {
		return mockContext.HttpClient
	})
	mockContext.Container.MustRegisterSingleton(func() exec.CommandRunner {
		return mockContext.CommandRunner
	})
	mockContext.Container.MustRegisterSingleton(func() input.Console {
		return mockContext.Console
	})
	mockContext.Container.MustRegisterSingleton(func() config.FileConfigManager {
		return mockContext.ConfigManager
	})
	mockContext.Container.MustRegisterSingleton(func() *alpha.FeatureManager {
		return mockContext.AlphaFeaturesManager
	})
	mockContext.Container.MustRegisterSingleton(func() *policy.ClientOptions {
		return mockContext.CoreClientOptions
	})
	mockContext.Container.MustRegisterSingleton(func() *arm.ClientOptions {
		return mockContext.ArmClientOptions
	})
}
