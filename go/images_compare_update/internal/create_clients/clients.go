package create_clients

import (
	"images_compare_update/global"
	"os"
	"sync"

	cr "github.com/alibabacloud-go/cr-20181201/v2/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// 定义了创建阿里云客户端和 Kubernetes 客户端的方法
type clientFactory interface {
	createAliyunClient() (*cr.Client, error)
	createK8sClientset() (*kubernetes.Clientset, error)
}

// 实现了 clientFactory 接口
type DefaultClientFactory struct{}

var _ clientFactory = &DefaultClientFactory{}

// 创建并返回阿里云客户端实例
func (d *DefaultClientFactory) createAliyunClient() (*cr.Client, error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(os.Getenv("ACCESS_KEY_ID")),
		AccessKeySecret: tea.String(os.Getenv("ACCESS_KEY_SECRET")),
		RegionId:        tea.String(global.ConfigStruct.GetRegionId()),
	}
	return cr.NewClient(config)
}

// 创建并返回 Kubernetes 客户端集实例
func (d *DefaultClientFactory) createK8sClientset() (*kubernetes.Clientset, error) {
	// 读取默认 kubeconfig 文件来创建 Kubernetes 配置
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(kubeconfig)
}

// 管理客户端实例的创建和获取，确保客户端的单例模式
type clientManager struct {
	aliyunClient     *cr.Client
	k8sClientset     *kubernetes.Clientset
	aliyunClientOnce sync.Once
	k8sClientsetOnce sync.Once
	factory          clientFactory
}

func NewClientManager(factory clientFactory) *clientManager {
	return &clientManager{factory: factory}
}

// 获取单例阿里云客户端实例
func (c *clientManager) GetAliyunClient() *cr.Client {
	c.aliyunClientOnce.Do(func() {
		client, err := c.factory.createAliyunClient()
		if err != nil || client == nil {
			global.Logger.Fatal("Failed to create Aliyun client", zap.Error(err))
		}
		c.aliyunClient = client
	})
	return c.aliyunClient
}

// 获取单例 Kubernetes 客户端集实例
func (c *clientManager) GetK8sClientset() *kubernetes.Clientset {
	c.k8sClientsetOnce.Do(func() {
		clientset, err := c.factory.createK8sClientset()
		if err != nil || clientset == nil {
			global.Logger.Fatal("Failed to create Kubernetes clientset", zap.Error(err))
		}
		c.k8sClientset = clientset
	})
	return c.k8sClientset
}
