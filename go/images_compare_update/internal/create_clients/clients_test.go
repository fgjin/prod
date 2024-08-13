package create_clients

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes"
	cr "github.com/alibabacloud-go/cr-20181201/v2/client"
)

// 模拟 clientFactory
type mockClientFactory struct {
	mock.Mock
}

func (m *mockClientFactory) createAliyunClient() (*cr.Client, error) {
	args := m.Called()
	return args.Get(0).(*cr.Client), args.Error(1)
}

func (m *mockClientFactory) createK8sClientset() (*kubernetes.Clientset, error) {
	args := m.Called()
	return args.Get(0).(*kubernetes.Clientset), args.Error(1)
}

func TestGetAliyunClient(t *testing.T) {
	mockFactory := new(mockClientFactory)
	client := &cr.Client{}
	mockFactory.On("createAliyunClient").Return(client, nil)

	manager := NewClientManager(mockFactory)
	result := manager.GetAliyunClient()

	assert.NotNil(t, result)
	assert.Equal(t, client, result)
	mockFactory.AssertExpectations(t)
}

func TestGetK8sClientset(t *testing.T) {
	mockFactory := new(mockClientFactory)
	clientset := &kubernetes.Clientset{}
	mockFactory.On("createK8sClientset").Return(clientset, nil)

	manager := NewClientManager(mockFactory)
	result := manager.GetK8sClientset()

	assert.NotNil(t, result)
	assert.Equal(t, clientset, result)
	mockFactory.AssertExpectations(t)
}

func TestGetAliyunClient_Error(t *testing.T) {
	mockFactory := new(mockClientFactory)
	mockFactory.On("createAliyunClient").Return(nil, errors.New("create aliyun client error"))

	manager := NewClientManager(mockFactory)

	assert.Panics(t, func() {
		manager.GetAliyunClient()
	})
}

func TestGetK8sClientset_Error(t *testing.T) {
	mockFactory := new(mockClientFactory)
	mockFactory.On("createK8sClientset").Return(nil, errors.New("create k8s clientset error"))

	manager := NewClientManager(mockFactory)

	assert.Panics(t, func() {
		manager.GetK8sClientset()
	})
}
