package ufile

import (
	"fmt"
	model "github.com/cloudreve/Cloudreve/v3/models"
	ufsdk "github.com/ufilesdk-dev/ufile-gosdk"
)

func NewClient(policy *model.Policy) (*ufsdk.UFileRequest, error){
	config := ufsdk.Config{
		PublicKey: policy.AccessKey,
		PrivateKey: policy.SecretKey,
		BucketName: policy.BucketName,
		FileHost: policy.BaseURL,
		BucketHost: policy.BaseURL,
		VerifyUploadMD5: false,
	}
	fmt.Println(config)
	u, err := ufsdk.NewFileRequest(&config, nil)

	if err != nil {
		return &ufsdk.UFileRequest{}, err
	}
	return u, err
}