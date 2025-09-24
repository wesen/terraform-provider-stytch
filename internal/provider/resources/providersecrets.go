package resources

// Provider-level B2B secrets exposed to resource code for precedence over env vars
var ProviderB2BLiveSecret string
var ProviderB2BTestSecret string

func SetProviderB2BSecrets(live, test string) {
    ProviderB2BLiveSecret = live
    ProviderB2BTestSecret = test
}


