package ufile

import (
	"context"
	"fmt"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/cache"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	ufsdk "github.com/ufilesdk-dev/ufile-gosdk"
	"testing"
)
func TestDriver_List(t *testing.T) {
	config := ufsdk.Config{
		PublicKey: "",
		PrivateKey: "",
		BucketName: "",
		FileHost: "",
		BucketHost: "",
		VerifyUploadMD5: false,
	}

	uf, err := ufsdk.NewFileRequest(&config, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	handler := Driver{
		Client: uf,
		HTTPClient: request.HTTPClient{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	list, err := handler.List(ctx, "cloud", false)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(list)
}

func TestDriver_Get(t *testing.T) {
	config := ufsdk.Config{
		PublicKey: "",
		PrivateKey: "",
		BucketName: "",
		FileHost: "",
		BucketHost: "",
		VerifyUploadMD5: false,
	}

	uf, err := ufsdk.NewFileRequest(&config, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	handler := Driver{
		Policy: &model.Policy{
			IsPrivate: true,
		},
		Client: uf,
		HTTPClient: request.HTTPClient{},
	}

	ctx := context.WithValue(context.Background(), fsctx.FileModelCtx, model.File{Size: 3, Name: "abc.txt"})

	cache.Set("setting_preview_timeout", "3600", 0)

	resp, err := handler.Get(ctx, "cloud123/")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("resp",resp)
}

func TestDriver_Token(t *testing.T) {
	config := ufsdk.Config{
		PublicKey: "",
		PrivateKey: "",
		BucketName: "",
		FileHost: "",
		BucketHost: "",
		VerifyUploadMD5: false,
	}

	uf, err := ufsdk.NewFileRequest(&config, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	handler := Driver{
		Policy: &model.Policy{
			IsPrivate: true,
		},
		Client: uf,
		HTTPClient: request.HTTPClient{},
	}
	cache.Set("setting_siteURL", "http://localhost:5212", 0)

	ctx := context.WithValue(context.Background(), fsctx.SavePathCtx, "test.txt")

	resp, err := handler.Token(ctx, 120, "")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(resp)
}

func TestDriver_CreateBucket(t *testing.T) {
	config := ufsdk.Config{
		PublicKey: "",
		PrivateKey: "",
		BucketName: "",
		FileHost: "",
		BucketHost: "",
		VerifyUploadMD5: false,
	}
	uf, err := ufsdk.NewBucketRequest(&config, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	handler := Driver{
		Client: uf,
		HTTPClient: request.HTTPClient{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = handler.CreateBucket(ctx, "yx2020", "cn-gd", "private", "")
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("create over")
}