package pls

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sdkErrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	vpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
	tccommon "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/common"
)

func TestIsVpcEndPointServiceWhiteListNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "short not found code",
			err:  sdkErrors.NewTencentCloudSDKError("ResourceNotFound", "not found", "request-id"),
			want: true,
		},
		{
			name: "qualified not found code",
			err:  sdkErrors.NewTencentCloudSDKError("InvalidParameterValue.ResourceNotFound", "not found", "request-id"),
			want: true,
		},
		{
			name: "transport error",
			err:  errors.New("transport error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isVpcEndPointServiceWhiteListNotFound(tt.err); got != tt.want {
				t.Fatalf("isVpcEndPointServiceWhiteListNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadEndPointServiceWhiteListTreatsMissingResourceAsDeleted(t *testing.T) {
	tests := []struct {
		name        string
		whiteList   *vpc.VpcEndPointServiceUser
		describeErr error
	}{
		{
			name:        "short not found error",
			describeErr: sdkErrors.NewTencentCloudSDKError("ResourceNotFound", "not found", "request-id"),
		},
		{
			name:        "qualified not found error",
			describeErr: sdkErrors.NewTencentCloudSDKError("InvalidParameterValue.ResourceNotFound", "not found", "request-id"),
		},
		{
			name: "empty describe result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newEndPointServiceWhiteListTestData(t)
			service := &fakeEndPointServiceWhiteListService{
				whiteList:   tt.whiteList,
				describeErr: tt.describeErr,
			}

			if err := readEndPointServiceWhiteList(context.Background(), d, service); err != nil {
				t.Fatalf("readEndPointServiceWhiteList() error = %v", err)
			}
			if d.Id() != "" {
				t.Fatalf("readEndPointServiceWhiteList() did not clear ID: %q", d.Id())
			}
		})
	}
}

func TestReadEndPointServiceWhiteListPropagatesUnexpectedError(t *testing.T) {
	d := newEndPointServiceWhiteListTestData(t)
	expected := errors.New("transport error")
	service := &fakeEndPointServiceWhiteListService{describeErr: expected}

	err := readEndPointServiceWhiteList(context.Background(), d, service)
	if !errors.Is(err, expected) {
		t.Fatalf("readEndPointServiceWhiteList() error = %v, want %v", err, expected)
	}
	if d.Id() == "" {
		t.Fatal("readEndPointServiceWhiteList() unexpectedly cleared ID")
	}
}

func TestDeleteEndPointServiceWhiteList(t *testing.T) {
	tests := []struct {
		name      string
		deleteErr error
		wantErr   error
	}{
		{
			name:      "not found is idempotent",
			deleteErr: sdkErrors.NewTencentCloudSDKError("InvalidParameterValue.ResourceNotFound", "not found", "request-id"),
		},
		{
			name:      "unexpected error is propagated",
			deleteErr: errors.New("permission denied"),
			wantErr:   errors.New("permission denied"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newEndPointServiceWhiteListTestData(t)
			service := &fakeEndPointServiceWhiteListService{deleteErr: tt.deleteErr}

			err := deleteEndPointServiceWhiteList(context.Background(), d, service)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("deleteEndPointServiceWhiteList() error = %v", err)
				}
			} else if err == nil || err.Error() != tt.wantErr.Error() {
				t.Fatalf("deleteEndPointServiceWhiteList() error = %v, want %v", err, tt.wantErr)
			}
			if service.userUin != "100000000001" || service.endPointServiceId != "vpcsvc-gone" {
				t.Fatalf(
					"deleteEndPointServiceWhiteList() parsed IDs = %q, %q",
					service.userUin,
					service.endPointServiceId,
				)
			}
		})
	}
}

func newEndPointServiceWhiteListTestData(t *testing.T) *schema.ResourceData {
	t.Helper()
	d := schema.TestResourceDataRaw(
		t,
		ResourceTencentCloudVpcEndPointServiceWhiteList().Schema,
		map[string]interface{}{},
	)
	d.SetId("100000000001" + tccommon.FILED_SP + "vpcsvc-gone")
	return d
}

type fakeEndPointServiceWhiteListService struct {
	whiteList         *vpc.VpcEndPointServiceUser
	describeErr       error
	deleteErr         error
	userUin           string
	endPointServiceId string
}

func (f *fakeEndPointServiceWhiteListService) DescribeVpcEndPointServiceWhiteListById(
	_ context.Context,
	userUin string,
	endPointServiceId string,
) (*vpc.VpcEndPointServiceUser, error) {
	f.userUin = userUin
	f.endPointServiceId = endPointServiceId
	return f.whiteList, f.describeErr
}

func (f *fakeEndPointServiceWhiteListService) DeleteVpcEndPointServiceWhiteListById(
	_ context.Context,
	userUin string,
	endPointServiceId string,
) error {
	f.userUin = userUin
	f.endPointServiceId = endPointServiceId
	return f.deleteErr
}
