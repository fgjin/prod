package update_image_online

import (
	"fmt"
	"images_compare_update/global"
	"testing"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

// 初始化测试环境
func init() {
	// global.Logger = zap.NewNop() // 不输出日志
	global.Logger, _ = zap.NewDevelopment() // 输出日志
}

func TestReplaceContainerImages(t *testing.T) {
	tests := []struct {
		name           string
		resourceType   string
		resourceName   string
		excludedImage  []string
		containers     *[]corev1.Container
		initContainers *[]corev1.Container
		acrAllImageMap map[string]string
		expectedUpdate bool
	}{
		{
			name:         "Image in excluded list",
			resourceType: "deployment",
			resourceName: "test-deployment",
			excludedImage: []string{
				"excluded-image",
			},
			containers: &[]corev1.Container{
				{Image: "harbor/ns/excluded-image:latest"},
			},
			initContainers: &[]corev1.Container{},
			acrAllImageMap: map[string]string{
				"excluded-image:latest": "acr/ns/excluded-image:latest",
			},
			expectedUpdate: false,
		},
		{
			name:         "Image not in ACR map",
			resourceType: "deployment",
			resourceName: "test-deployment",
			excludedImage: []string{
				"excluded-image",
			},
			containers: &[]corev1.Container{
				{Image: "harbor/ns/test-image:latest"},
			},
			initContainers: &[]corev1.Container{},
			acrAllImageMap: map[string]string{
				"test-image1:latest": "acr/ns/test-image1:latest",
			},
			expectedUpdate: false,
		},
		{
			name:         "Image updated successfully",
			resourceType: "deployment",
			resourceName: "test-deployment",
			excludedImage: []string{
				"excluded-image",
			},
			containers: &[]corev1.Container{
				{Image: "harbor/ns/test-image:latest"},
			},
			initContainers: &[]corev1.Container{
				{Image: "harbor/ns/test-init-image:latest"},
			},
			acrAllImageMap: map[string]string{
				"test-image:latest":      "acr/ns/test-image:latest",
				"test-init-image:latest": "acr/ns/test-init-image:latest",
			},
			expectedUpdate: true,
		},
		{
			name:         "Image already up to date",
			resourceType: "deployment",
			resourceName: "test-deployment",
			excludedImage: []string{
				"excluded-image",
			},
			containers: &[]corev1.Container{
				{Image: "acr/ns/test-image:latest"},
			},
			initContainers: &[]corev1.Container{},
			acrAllImageMap: map[string]string{
				"test-image:latest": "acr/ns/test-image:latest",
			},
			expectedUpdate: false,
		},
	}
	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Println(tt.name)
			updated := replaceContainerImages(tt.resourceType, tt.resourceName, tt.excludedImage, tt.containers, tt.initContainers, tt.acrAllImageMap)
			if updated != tt.expectedUpdate {
				t.Errorf("expected update to be %v, got %v", tt.expectedUpdate, updated)
			}
		})
	}
}
