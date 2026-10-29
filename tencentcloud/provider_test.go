package tencentcloud

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"os"
	"testing"

	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/connectivity"
)

func init() {
	envProject := os.Getenv("QCI_JOB_ID")
	envNum := os.Getenv("QCI_BUILD_NUMBER")
	envId := os.Getenv("QCI_BUILD_ID")
	reqCli := fmt.Sprintf("Terraform-%s/%s-%s", envProject, envNum, envId)
	_ = os.Setenv(connectivity.REQUEST_CLIENT, reqCli)
}

func TestProvider(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProviderImpl(t *testing.T) {
	var _ = Provider()
}

func TestProviderWebIdentityTokenFile(t *testing.T) {
	for _, tt := range []struct {
		name, contents, want string
		wantErr              bool
	}{
		{name: "TKE JWT", contents: "header.payload.signature", want: "header.payload.signature"},
		{name: "JWT with newline", contents: "  header.payload.signature\n", want: "header.payload.signature"},
		{name: "legacy JSON", contents: `{"web_identity_token":"legacy-token"}`, want: "legacy-token"},
		{name: "empty file", wantErr: true},
		{name: "empty JSON token", contents: `{"web_identity_token":" "}`, wantErr: true},
		{name: "missing JSON token", contents: `{}`, wantErr: true},
		{name: "malformed JSON", contents: "{", wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "token")
			if err := os.WriteFile(path, []byte(tt.contents), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := getConfigFromWebIdentityTokenFile(path)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected token file error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got["web_identity_token"] != tt.want {
				t.Fatalf("token = %q, want %q", got["web_identity_token"], tt.want)
			}
			if err := os.WriteFile(path, []byte("rotated.payload.signature"), 0600); err != nil {
				t.Fatal(err)
			}
			got, err = getConfigFromWebIdentityTokenFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got["web_identity_token"] != "rotated.payload.signature" {
				t.Fatal("token rotation was not read")
			}
		})
	}
	if _, err := getConfigFromWebIdentityTokenFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected missing token file error")
	}
}

type providerRoundTripFunc func(*http.Request) (*http.Response, error)

func (f providerRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProviderOIDCAssumeRoleChain(t *testing.T) {
	providerSchema := Provider().Schema
	for _, tt := range []struct {
		name             string
		static, oidcOnly bool
		rejectAction     string
	}{
		{name: "OIDC then AssumeRole"},
		{name: "OIDC only", oidcOnly: true},
		{name: "static AssumeRole unchanged", static: true},
		{name: "OIDC failure stops chain", rejectAction: "AssumeRoleWithWebIdentity"},
		{name: "AssumeRole failure is returned", rejectAction: "AssumeRole"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, env := range []string{PROVIDER_SECRET_ID, PROVIDER_SECRET_KEY, PROVIDER_SECURITY_TOKEN,
				PROVIDER_CAM_ROLE_NAME, PROVIDER_ASSUME_ROLE_ARN, PROVIDER_ASSUME_ROLE_SESSION_NAME,
				PROVIDER_ASSUME_ROLE_WEB_IDENTITY_TOKEN, PROVIDER_ASSUME_ROLE_SAML_ASSERTION,
				PROVIDER_MFA_CERTIFICATION_SERIAL_NUMBER, PROVIDER_MFA_CERTIFICATION_TOKEN_CODE} {
				t.Setenv(env, "")
			}
			// Isolate local tccli profiles; never use the developer's credentials.
			previousProfile := providerConfig
			providerConfig = make(map[string]interface{})
			t.Cleanup(func() { providerConfig = previousProfile })
			const oidcRole = "qcs::cam::uin/100000000001:roleName/controlplane"
			const shootRole = "qcs::cam::uin/200000000002:roleName/dataplane"
			path := filepath.Join(t.TempDir(), "token")
			if err := os.WriteFile(path, []byte("header.payload.signature\n"), 0600); err != nil {
				t.Fatal(err)
			}
			raw := map[string]interface{}{"region": "ap-guangzhou"}
			if tt.static {
				raw["secret_id"] = "static-id"
				raw["secret_key"] = "static-key"
			} else {
				raw["assume_role_with_web_identity"] = []interface{}{map[string]interface{}{
					"provider_id": "seed-cluster", "role_arn": oidcRole,
					"session_name": "controlplane", "session_duration": 3600,
					"web_identity_token_file": path,
				}}
			}
			if !tt.oidcOnly {
				raw["assume_role"] = []interface{}{map[string]interface{}{
					"role_arn": shootRole, "session_name": "dataplane",
					"session_duration": 1800, "external_id": "expected-external-id",
				}}
			}
			var actions []string
			previousTransport := http.DefaultTransport
			http.DefaultTransport = providerRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				action := r.Header.Get("X-TC-Action")
				actions = append(actions, action)
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					return nil, err
				}
				id, key, token := "oidc-id", "oidc-key", "oidc-token"
				switch action {
				case "AssumeRoleWithWebIdentity":
					if len(actions) != 1 {
						t.Error("OIDC must be the first STS call")
					}
					if body["RoleArn"] != oidcRole || body["ProviderId"] != "seed-cluster" || body["WebIdentityToken"] != "header.payload.signature" {
						t.Errorf("incorrect OIDC request: %#v", body)
					}
					if r.Header.Get("Authorization") != "SKIP" {
						t.Error("OIDC request must not use static credentials")
					}
				case "AssumeRole":
					wantID, wantToken := "oidc-id", "oidc-token"
					if tt.static {
						wantID, wantToken = "static-id", ""
					}
					if !strings.Contains(r.Header.Get("Authorization"), "Credential="+wantID+"/") || r.Header.Get("X-TC-Token") != wantToken {
						t.Error("AssumeRole was not signed with the expected base credentials")
					}
					if body["RoleArn"] != shootRole || body["RoleSessionName"] != "dataplane" || body["ExternalId"] != "expected-external-id" || body["DurationSeconds"] != float64(1800) {
						t.Errorf("incorrect chained AssumeRole request: %#v", body)
					}
					id, key, token = "shoot-id", "shoot-key", "shoot-token"
				default:
					return nil, fmt.Errorf("unexpected cloud API call %s", action)
				}
				response := fmt.Sprintf(`{"Response":{"Credentials":{"TmpSecretId":%q,"TmpSecretKey":%q,"Token":%q},"RequestId":"test-request"}}`, id, key, token)
				if action == tt.rejectAction {
					response = `{"Response":{"Error":{"Code":"AuthFailure","Message":"test rejection"},"RequestId":"test-request"}}`
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response))}, nil
			})
			t.Cleanup(func() { http.DefaultTransport = previousTransport })
			meta, err := providerConfigure(schema.TestResourceDataRaw(t, providerSchema, raw))
			if tt.rejectAction != "" {
				if err == nil || !strings.Contains(err.Error(), "test rejection") {
					t.Fatalf("expected STS rejection, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				cred := meta.(*TencentCloudClient).apiV3Conn.Credential
				wantID, wantKey, wantToken := "shoot-id", "shoot-key", "shoot-token"
				if tt.oidcOnly {
					wantID, wantKey, wantToken = "oidc-id", "oidc-key", "oidc-token"
				}
				if cred.SecretId != wantID || cred.SecretKey != wantKey || cred.Token != wantToken {
					t.Fatal("provider retained the wrong final credentials")
				}
			}
			wantActions := "AssumeRoleWithWebIdentity,AssumeRole"
			if tt.static {
				wantActions = "AssumeRole"
			}
			if tt.oidcOnly || tt.rejectAction == "AssumeRoleWithWebIdentity" {
				wantActions = "AssumeRoleWithWebIdentity"
			}
			if strings.Join(actions, ",") != wantActions {
				t.Fatalf("STS calls = %v, want %s", actions, wantActions)
			}
		})
	}
}
