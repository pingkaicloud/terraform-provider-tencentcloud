package pls

import (
	"errors"
	"testing"

	sdkErrors "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
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
