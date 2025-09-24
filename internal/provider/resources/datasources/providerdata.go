package datasources

import (
	"github.com/stytchauth/stytch-management-go/v2/pkg/api"
)

// ProviderData is stored in DataSourceData to expose both the management client
// (for potential cross-calls) and the optional B2B secrets for building auth SDK clients.
type ProviderData struct {
	ManagementClient *api.API
	B2BLiveSecret    string
	B2BTestSecret    string
}

func NewProviderData(mgmt *api.API, live, test string) *ProviderData {
	return &ProviderData{ManagementClient: mgmt, B2BLiveSecret: live, B2BTestSecret: test}
}
