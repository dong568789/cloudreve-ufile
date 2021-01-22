package ufile

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	model "github.com/cloudreve/Cloudreve/v3/models"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/fsctx"
	"github.com/cloudreve/Cloudreve/v3/pkg/filesystem/response"
	"github.com/cloudreve/Cloudreve/v3/pkg/request"
	"github.com/cloudreve/Cloudreve/v3/pkg/serializer"
	"github.com/gin-gonic/gin"
	ufsdk "github.com/ufilesdk-dev/ufile-gosdk"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Uploadpolicy struct {
	Expiration string        `json:"expiration"`
	Conditions []interface{} `json:"conditions"`
}

// CallbackPolicy 回调策略
type CallbackPolicy struct {
	CallbackURL      string `json:"callbackUrl"`
	CallbackBody     string `json:"callbackBody"`
}

// Driver Ucloud ufile适配器模板
type Driver struct {
	Policy     *model.Policy
	Client     *ufsdk.UFileRequest
	HTTPClient request.Client
}

func (handler Driver)List(ctx context.Context, base string, recursive bool) ([]response.Object, error) {
	var (
		marker string
		objects []ufsdk.ObjectInfo
		commons []ufsdk.CommonPreInfo
	)

	prefix := strings.TrimPrefix(base, "/")

	for true {
		res, err := handler.Client.ListObjects(prefix, marker,"",1000)
		fmt.Println("res:", res)

		if err != nil {
			return nil, err
		}

		objects = append(objects, res.Contents...)
		commons = append(commons, res.CommonPrefixes...)

		marker = res.NextMarker
		if marker == "" {
			break
		}
	}

	res := make([]response.Object, 0, len(objects) + len(commons))

	for _, object := range commons{
		rel, err := filepath.Rel(prefix, object.Prefix)
		if err != nil {
			continue
		}
		res = append(res, response.Object{
			Name: path.Base(object.Prefix),
			RelativePath: filepath.ToSlash(rel),
			Size: 0,
			IsDir: true,
			LastModify: time.Now(),
		})
	}

	for _, object := range objects{
		rel, err := filepath.Rel(prefix, object.Key)
		if err != nil {
			continue
		}
		size, _ := strconv.ParseUint(object.Key, 10, 64)
		res = append(res, response.Object{
			Name: path.Base(object.Key),
			RelativePath: filepath.ToSlash(rel),
			Size: size,
			Source: object.Key,
			IsDir: false,
			LastModify: time.Now(),
		})
	}
	return res, nil
}

func (handler Driver)Get(ctx context.Context, path string) (response.RSCloser, error) {
	downloadUrl, err := handler.Source(
		ctx,
		path,
		url.URL{},
		int64(model.GetIntSetting("preview_timeout", 60)),
		false,
		0,
	)
	if err != nil {
		return nil, err
	}

	fmt.Println("downloadUrl", downloadUrl)
	resp, err := handler.HTTPClient.Request(
		"GET",
		downloadUrl,
		nil,
		request.WithContext(ctx),
		request.WithTimeout(time.Duration(0)),
	).CheckHTTPResponse(200).GetRSCloser()

	resp.SetFirstFakeChunk()

	if file, ok := ctx.Value(fsctx.FileModelCtx).(model.File); ok {
		resp.SetContentLength(int64(file.Size))
	}
	return resp, nil
}

func (handler Driver)Put(ctx context.Context, file io.ReadCloser, dst string, size uint64) error {

	return handler.Client.IOPut(file, dst, "")
}

func (handler Driver)Delete(ctx context.Context, files []string) ([]string, error) {
	var (
		failed 		 = make([]string, 0, len(files))
		lastErr 	 error
		currentIndex = 0
		indexLock    sync.Mutex
		failedLock 	 sync.Mutex
		wg           sync.WaitGroup
		routineNum 	 = 4
	)
	wg.Add(routineNum)

	//ufile不支持批量删除，这里开4个携程并行操作
	for i := 0; i < routineNum; i++ {
		go func() {
			for {
				indexLock.Lock()
				if currentIndex >= len(files) {
					wg.Done()
					indexLock.Unlock()
					return
				}
				path := files[currentIndex]
				currentIndex++
				indexLock.Unlock()

				err := handler.Client.DeleteFile(path)
				if err != nil {
					failedLock.Lock()
					lastErr = err
					failed = append(failed, path)
					failedLock.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	return failed, lastErr
}

// Thumb 获取文件缩略图
func (handler Driver) Thumb(ctx context.Context, path string) (*response.ContentResponse, error) {
	return nil, errors.New("未实现")
}

func (handler Driver)Source(ctx context.Context, path string, baseUrl url.URL, ttl int64, isDownload bool, speed int) (string, error) {
	//fileName := ""
	//if file, ok := ctx.Value(fsctx.FileModelCtx).(model.File); ok {
	//	fileName = file.Name
	//}
	//path = path + fileName
	fmt.Println("fileName:", path)
	return handler.signSourceUrl(ctx, path, ttl)
}

func (handler Driver)signSourceUrl(ctx context.Context, path string, ttl int64) (string, error) {
	if !handler.Policy.IsPrivate {
		sourceURL := handler.Client.GetPublicURL(path)
		return sourceURL, nil
	}

	fmt.Println("path:", path)

	sourceURL := handler.Client.GetPrivateURL(path, time.Duration(ttl)*time.Minute)
	fmt.Println("sourceURL:", sourceURL)
	return sourceURL, nil
}

func (handler Driver)Token(ctx context.Context, TTL int64, key string) (serializer.UploadCredential, error) {
	// 生成回调地址
	siteURL := model.GetSiteURL()
	apiBaseURI, _ := url.Parse("/api/v3/callback/ufile/" + key)
	//apiBaseURI, _ := url.Parse("http://a.yxgames.com/iqy/data?key=" + key)
	apiURL := siteURL.ResolveReference(apiBaseURI)

	fmt.Println("apiURL", apiURL.String())
	// 读取上下文中生成的存储路径
	savePath, ok := ctx.Value(fsctx.SavePathCtx).(string)
	if !ok {
		return serializer.UploadCredential{}, errors.New("无法获取存储路径")
	}

	size, ok := ctx.Value(fsctx.FileSizeCtx).(uint64)
	if !ok {
		return serializer.UploadCredential{}, errors.New("文件大小错误")
	}

	// 回调策略
	callbackPolicy := CallbackPolicy{
		CallbackURL: apiURL.String(),
		CallbackBody: fmt.Sprintf("name=%s&source_name=%s&size=%d&pic_info=%s&key=%s", "", savePath, size, "", key),
	}

	putPolicy := Uploadpolicy{
		Expiration: time.Now().UTC().Add(time.Duration(TTL) * time.Second).Format(time.RFC3339),
	}

	return handler.getUploadCredential(ctx, putPolicy, callbackPolicy)
}

func (handler Driver)getUploadCredential(ctx context.Context, policy Uploadpolicy, callbackPolicy CallbackPolicy) (serializer.UploadCredential, error) {
	// 读取上下文中生成的存储路径和文件大小
	savePath, ok := ctx.Value(fsctx.SavePathCtx).(string)
	if !ok {
		return serializer.UploadCredential{}, errors.New("无法获取存储路径")
	}

	c, _ := ctx.Value(fsctx.GinCtx).(*gin.Context)

	callbackPolicyEncoded := ""
	if callbackPolicy.CallbackURL != "" {
		callbackPolicyJSON, err := json.Marshal(callbackPolicy)
		if err != nil {
			return serializer.UploadCredential{}, err
		}
		fmt.Println("serializer", string(callbackPolicyJSON))
		callbackPolicyEncoded = base64.URLEncoding.EncodeToString(callbackPolicyJSON)
	}
	contentType := c.Request.Header.Get("Content-Type")

	if contentType == "" {
		switch strings.ToLower(path.Ext(savePath)) {
		case ".rar":
			c.Request.Header.Set("Content-Type", "application/octet-stream")
		default:
			c.Request.Header.Set("Content-Type", "application/octet-stream")
		}
	}

	//fmt.Println("ext:", path.Ext(savePath))
	//fmt.Println("Content-MD5:" , c.Request.Header.Get("Content-MD5"))
	//fmt.Println("Content-type:" , c.Request.Header.Get("Content-Type"))
	//fmt.Println("savePath:" , savePath)
	//fmt.Println("BucketName:" , handler.Policy.BucketName)
	//fmt.Println("callbackPolicyEncoded:" , callbackPolicyEncoded)
	putPolicy := handler.Client.Auth.AuthorizationPolicy("POST", handler.Policy.BucketName, savePath, callbackPolicyEncoded, c.Request.Header)
	return serializer.UploadCredential{
		Policy:    putPolicy,
		Path:      savePath,
		AccessKey: handler.Policy.AccessKey,
		Token:     putPolicy,
	}, nil
}

func (handler Driver)CreateBucket(ctx context.Context, bucketName string, region string, bucketType string, projectId string) error{
	_, err := handler.Client.CreateBucket(bucketName, region, bucketType, projectId)
	if err != nil {
		return err
	}
	
	return nil
}