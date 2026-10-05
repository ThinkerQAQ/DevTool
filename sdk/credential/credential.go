package credential

const (
	ServiceName      = "credential"
	StoreServiceName = "credential-store"

	MethodResolve = "resolve"
	MethodGet     = "get"
	MethodPut     = "put"
)

type ResolveRequest struct {
	Root   string   `json:"root,omitempty"`
	Host   string   `json:"host"`
	Scopes []string `json:"scopes,omitempty"`
}

type Authorization struct {
	Status          string `json:"status"`
	VerificationURI string `json:"verification_uri,omitempty"`
	UserCode        string `json:"user_code,omitempty"`
	ExpiresAt       int64  `json:"expires_at,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

type ResolveResponse struct {
	Ready         bool           `json:"ready"`
	Secret        string         `json:"secret,omitempty"`
	Source        string         `json:"source,omitempty"`
	Authorization *Authorization `json:"authorization,omitempty"`
}

type StoreGetRequest struct {
	Key string `json:"key"`
}

type StoreGetResponse struct {
	Found bool   `json:"found"`
	Value string `json:"value,omitempty"`
}

type StorePutRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
