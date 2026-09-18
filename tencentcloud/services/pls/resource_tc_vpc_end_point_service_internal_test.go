package pls

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sdkErrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	vpc "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/vpc/v20170312"
)

func TestIsVpcEndPointServiceNotFound(t *testing.T) {
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
			if got := isVpcEndPointServiceNotFound(tt.err); got != tt.want {
				t.Fatalf("isVpcEndPointServiceNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadEndPointServiceTreatsMissingResourceAsDeleted(t *testing.T) {
	tests := []struct {
		name        string
		service     *vpc.EndPointService
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
			d := newEndPointServiceTestData(t)
			service := &fakeEndPointServiceService{
				endPointService: tt.service,
				describeErr:     tt.describeErr,
			}

			if err := readEndPointService(context.Background(), d, service); err != nil {
				t.Fatalf("readEndPointService() error = %v", err)
			}
			if d.Id() != "" {
				t.Fatalf("readEndPointService() did not clear ID: %q", d.Id())
			}
			if service.endPointServiceID != "vpcsvc-gone" {
				t.Fatalf("readEndPointService() id = %q, want %q", service.endPointServiceID, "vpcsvc-gone")
			}
		})
	}
}

func TestReadEndPointServicePropagatesUnexpectedError(t *testing.T) {
	d := newEndPointServiceTestData(t)
	expected := errors.New("transport error")
	service := &fakeEndPointServiceService{describeErr: expected}

	err := readEndPointService(context.Background(), d, service)
	if !errors.Is(err, expected) {
		t.Fatalf("readEndPointService() error = %v, want %v", err, expected)
	}
	if d.Id() == "" {
		t.Fatal("readEndPointService() unexpectedly cleared ID")
	}
}

func newEndPointServiceTestData(t *testing.T) *schema.ResourceData {
	t.Helper()
	d := schema.TestResourceDataRaw(
		t,
		ResourceTencentCloudVpcEndPointService().Schema,
		map[string]interface{}{},
	)
	d.SetId("vpcsvc-gone")
	return d
}

type fakeEndPointServiceService struct {
	endPointService   *vpc.EndPointService
	describeErr       error
	endPointServiceID string
}

func (f *fakeEndPointServiceService) DescribeVpcEndPointServiceById(
	_ context.Context,
	endPointServiceId string,
) (*vpc.EndPointService, error) {
	f.endPointServiceID = endPointServiceId
	return f.endPointService, f.describeErr
}
