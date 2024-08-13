package get_images

import (
	"context"
	"strings"
	"sync"
	"time"

	"images_compare_update/global"
	"images_compare_update/internal/create_clients"
	"images_compare_update/pkg/utils"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	concurrency    = 10   // 并发大小
	k8sImageChCaps = 1000 // 存放待处理镜像容量
)

// 获取 k8s 所有服务的镜像
func GetK8sImages(timeout int) ([]string, map[string]string) {
	clientManager := create_clients.NewClientManager(&create_clients.DefaultClientFactory{})
	clientset := clientManager.GetK8sClientset()
	imageCh := make(chan string, k8sImageChCaps)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	wg.Add(1)
	go batchProcessingPods(ctx, clientset, imageCh, &wg)

	// 等待协程处理完并关闭 channel
	go func() {
		wg.Wait()
		close(imageCh)
	}()

	imageSli := make([]string, 0, k8sImageChCaps)
	for image := range imageCh {
		imageSli = append(imageSli, image)
	}

	// 切片去重
	uniqueImageSli := utils.RemoveSliceDuplicates(imageSli)
	// 将切片转换成 map
	k8sImageMap := utils.ConvertSliceToMap(uniqueImageSli,
		func(image string) string {
			splitImageToSlice := strings.Split(image, "/")
			if len(splitImageToSlice) > 1 {
				return splitImageToSlice[len(splitImageToSlice)-1]
			}
			return image
		},
		func(image string) string { return image },
	)

	k8sImageLastPartSli := make([]string, 0, len(uniqueImageSli))
	for _, v := range uniqueImageSli {
		parts := strings.Split(v, "/")
		lastPart := parts[len(parts)-1]
		k8sImageLastPartSli = append(k8sImageLastPartSli, lastPart)
	}

	global.Logger.Debug("Logging 'k8sImageLastPartSli'", zap.Any("k8sImageLastPartSli", k8sImageLastPartSli), zap.Int("len", len(k8sImageLastPartSli)))
	global.Logger.Debug("Logging 'k8sImageMap'", zap.Any("k8sImageMap", k8sImageMap), zap.Int("len", len(k8sImageMap)))

	return k8sImageLastPartSli, k8sImageMap
}

// 分批次处理 Pod
func batchProcessingPods(ctx context.Context, clientset *kubernetes.Clientset, imageCh chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()

	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		global.Logger.Error("Failed to get pods", zap.Error(err))
		return
	}

	podCh := make(chan []corev1.Pod, concurrency)
	podWg := sync.WaitGroup{}

	// 计算批次大小
	batchSize := (len(pods.Items) + concurrency - 1) / concurrency

	// 将 Pod 分批发送到 podCh
	for i := 0; i < len(pods.Items); i += batchSize {
		end := i + batchSize
		if end > len(pods.Items) {
			end = len(pods.Items)
		}
		podCh <- pods.Items[i:end]
	}
	close(podCh)

	// 启动多个协程处理
	for i := 0; i < concurrency; i++ {
		podWg.Add(1)
		go func() {
			defer podWg.Done()
			for podBatchSli := range podCh {
				sendImageToChannel(podBatchSli, imageCh)
			}
		}()
	}

	podWg.Wait()
}

// 将镜像发送到 channel，包括容器和初始化容器
func sendImageToChannel(podBatchSli []corev1.Pod, imageCh chan<- string) {
	for _, pod := range podBatchSli {
		// 处理常规容器的镜像
		for _, container := range pod.Spec.Containers {
			imageCh <- container.Image
		}
		// 处理初始化容器的镜像
		if pod.Spec.InitContainers != nil {
			for _, initContainer := range pod.Spec.InitContainers {
				imageCh <- initContainer.Image
			}
		}
	}
}
