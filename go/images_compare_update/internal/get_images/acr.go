package get_images

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"images_compare_update/global"
	"images_compare_update/internal/create_clients"

	cr "github.com/alibabacloud-go/cr-20181201/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

const (
	acrImageChCaps = 300 // 存放待处理镜像容量
	pageSize       = 50  // 每页返回数量
	retryNum       = 3   // 重试次数
	retryInterval  = 1   // 重试间隔
	requestLimit   = 5   // 每秒最多请求数
	bucket         = 100 // 桶容量
)

// 从 ACR 获取所有镜像
func GetAcrImages(acrAddr, instanceId string, semaphoreCap, timeout int) ([]string, map[string]string) {
	clientManager := create_clients.NewClientManager(&create_clients.DefaultClientFactory{})
	client := clientManager.GetAliyunClient()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// 速率限制器
	limiter := rate.NewLimiter(rate.Limit(requestLimit), bucket)

	// 获取所有仓库
	allRepoSli, err := listAllRepos(ctx, client, instanceId, limiter)
	if err != nil {
		global.Logger.Error("Failed to exec listAllRepos", zap.Error(err))
		return nil, nil
	}

	// 用于存储镜像信息
	imageCh := make(chan string, acrImageChCaps)
	// 使用信号量控制并发数量
	semaphoreCh := make(chan struct{}, semaphoreCap)

	wg := sync.WaitGroup{}
	for _, repo := range allRepoSli {
		wg.Add(1)
		go func(repo *cr.ListRepositoryResponseBodyRepositories) {
			defer wg.Done()

			// 获取信号量
			semaphoreCh <- struct{}{}
			// 释放信号量
			defer func() { <-semaphoreCh }()

			err := retryOperation(func() error {
				return listRepoAllImages(ctx, client, acrAddr, instanceId, repo, imageCh, limiter)
			}, retryNum, ctx)
			if err != nil {
				global.Logger.Error("Failed to list images", zap.String("RepoName", tea.StringValue(repo.RepoName)), zap.Error(err))
			}
		}(repo)
	}

	// 等待协程处理完并关闭 channel
	go func() {
		wg.Wait()
		close(imageCh)
	}()

	acrImageSli := make([]string, 0, acrImageChCaps)
	acrImageMap := make(map[string]string, acrImageChCaps)
	for image := range imageCh {
		parts := strings.Split(image, "/")
		lastPart := parts[len(parts)-1]
		acrImageSli = append(acrImageSli, lastPart)
		acrImageMap[lastPart] = image
	}

	global.Logger.Debug("Logging 'acrImageSli'", zap.Any("acrImageSli", acrImageSli), zap.Int("len", len(acrImageSli)))
	global.Logger.Debug("Logging 'acrImageMap'", zap.Any("acrImageMap", acrImageMap), zap.Int("len", len(acrImageMap)))

	return acrImageSli, acrImageMap
}

// 获取 ACR 所有仓库
func listAllRepos(ctx context.Context, client *cr.Client, instanceId string, limiter *rate.Limiter) ([]*cr.ListRepositoryResponseBodyRepositories, error) {
	allRepoSli := make([]*cr.ListRepositoryResponseBodyRepositories, 0)
	pageNo := int32(1)

	for {
		var resp *cr.ListRepositoryResponse
		err := retryOperation(func() error {
			req := &cr.ListRepositoryRequest{
				InstanceId: tea.String(instanceId),
				PageNo:     tea.Int32(pageNo),
				PageSize:   tea.Int32(int32(pageSize)),
			}

			if err := limiter.Wait(ctx); err != nil {
				return fmt.Errorf("rate limiter wait error: %v", err)
			}

			var err error
			resp, err = client.ListRepository(req)
			if err != nil {
				return fmt.Errorf("failed to list repos: %v", err)
			}
			return nil
		}, retryNum, ctx)

		if err != nil {
			return nil, err
		}

		if resp.Body == nil || len(resp.Body.Repositories) == 0 {
			break
		}

		allRepoSli = append(allRepoSli, resp.Body.Repositories...)

		// 如果返回的数量小于页面大小，说明已经获取了所有的数据
		if len(resp.Body.Repositories) < int(pageSize) {
			break
		}

		pageNo++
	}

	return allRepoSli, nil
}

// 获取仓库中的所有镜像
func listRepoAllImages(ctx context.Context, client *cr.Client, acrAddr, instanceId string, repo *cr.ListRepositoryResponseBodyRepositories, imageChan chan<- string, limiter *rate.Limiter) error {
	pageNo := int32(1)

	for {
		req := &cr.ListRepoTagRequest{
			InstanceId: tea.String(instanceId),
			RepoId:     repo.RepoId,
			PageNo:     tea.Int32(pageNo),
			PageSize:   tea.Int32(int32(pageSize)),
		}

		var resp *cr.ListRepoTagResponse

		operation := func() error {
			if err := limiter.Wait(ctx); err != nil {
				return fmt.Errorf("rate limiter wait error: %v", err)
			}

			var err error
			resp, err = client.ListRepoTag(req)
			if err != nil {
				return fmt.Errorf("failed to list RepoTag: %v", err)
			}
			return nil
		}

		err := retryOperation(operation, retryNum, ctx)
		if err != nil {
			return err
		}

		if resp.Body == nil || len(resp.Body.Images) == 0 {
			global.Logger.Warn("No images found in repos", zap.String("RepoName", tea.StringValue(repo.RepoName)))
			break
		}

		// 将镜像发送到 channel
		for _, image := range resp.Body.Images {
			select {
			case <-ctx.Done():
				return fmt.Errorf("request timed out while sending image: %v", ctx.Err())
			case imageChan <- fmt.Sprintf("%s/%s/%s:%s", tea.StringValue(&acrAddr), tea.StringValue(repo.RepoNamespaceName), tea.StringValue(repo.RepoName), tea.StringValue(image.Tag)):
			}
		}

		// 如果返回的数量小于页面大小，说明已经获取了所有的数据
		if len(resp.Body.Images) < int(pageSize) {
			break
		}

		pageNo++
	}

	return nil
}

// 重试逻辑
func retryOperation(operation func() error, retryCount int, ctx context.Context) error {
	var err error
	for i := 0; i < retryCount; i++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("operation timed out: %v", ctx.Err())
		default:
			err = operation()
			if err == nil {
				return nil
			}
			if strings.Contains(err.Error(), "Throttling") {
				global.Logger.Debug("Throttling detected, retrying", zap.Int("attempt", i+1))
				time.Sleep(retryInterval * time.Second)
			} else {
				return fmt.Errorf("operation failed: %v", err)
			}
		}
	}
	return fmt.Errorf("operation failed after retries: %v", err)
}
