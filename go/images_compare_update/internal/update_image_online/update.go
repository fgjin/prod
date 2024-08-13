package update_image_online

import (
	"context"
	"strings"
	"sync"
	"time"

	"images_compare_update/global"
	"images_compare_update/internal/create_clients"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	batchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
)

// 更新资源相关字段
func UpdateResourceFields(updateConcurrency, updateWaitTime int, excludedImage, namespaceSli []string, acrAllImageMap map[string]string) {
	clientManager := create_clients.NewClientManager(&create_clients.DefaultClientFactory{})
	clientset := clientManager.GetK8sClientset()
	factory := &updaterFactory{clientset: clientset}
	resourceCh := make(chan struct{}, updateConcurrency)
	var wg sync.WaitGroup

	for _, ns := range namespaceSli {
		for _, resourceType := range []string{"deployment", "statefulset", "daemonset", "cronjob"} {
			updater := factory.newUpdater(resourceType, ns)
			resourceCh <- struct{}{}
			wg.Add(1)
			go func(updateWaitTime int, updater resourceUpdater, ns, resourceType string) {
				defer wg.Done()
				defer func() { <-resourceCh }()
				err := updater.updateResource(updateWaitTime, resourceType, excludedImage, acrAllImageMap)
				if err != nil {
					if errors.IsNotFound(err) {
						global.Logger.Debug("Resource not found, skipping", zap.String("namespace", ns), zap.String("resourceType", resourceType))
					} else {
						global.Logger.Error("Failed to update resource", zap.String("namespace", ns), zap.String("resourceType", resourceType), zap.Error(err))
					}
				}
			}(updateWaitTime, updater, ns, resourceType)
		}
	}

	wg.Wait()
}

// 资源更新接口
type resourceUpdater interface {
	updateResource(updateWaitTime int, resourceType string, excludedImage []string, acrAllImageMap map[string]string) error
}

// Deployment 更新实现
type deploymentUpdater struct {
	client appsv1.DeploymentInterface
}

func (d *deploymentUpdater) updateResource(updateWaitTime int, resourceType string, excludedImage []string, acrAllImageMap map[string]string) error {
	deployments, err := d.client.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, deployment := range deployments.Items {
		if ok := replaceContainerImages(resourceType, deployment.Name, excludedImage, &deployment.Spec.Template.Spec.Containers, &deployment.Spec.Template.Spec.InitContainers, acrAllImageMap); ok {
			deployment.Spec.Template.Spec.ImagePullSecrets = nil
			if _, err := d.client.Update(context.Background(), &deployment, metav1.UpdateOptions{}); err != nil {
				global.Logger.Error("Failed to update deployment", zap.String("deployment", deployment.Name), zap.Error(err))
			} else {
				global.Logger.Info("Successfully updated deployment", zap.String("deployment", deployment.Name))
			}
			time.Sleep(time.Duration(updateWaitTime) * time.Minute)
		}
	}
	return nil
}

// StatefulSet 更新实现
type statefulSetUpdater struct {
	client appsv1.StatefulSetInterface
}

func (s *statefulSetUpdater) updateResource(updateWaitTime int, resourceType string, excludedImage []string, acrAllImageMap map[string]string) error {
	statefulSets, err := s.client.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, statefulSet := range statefulSets.Items {
		if ok := replaceContainerImages(resourceType, statefulSet.Name, excludedImage, &statefulSet.Spec.Template.Spec.Containers, &statefulSet.Spec.Template.Spec.InitContainers, acrAllImageMap); ok {
			statefulSet.Spec.Template.Spec.ImagePullSecrets = nil
			if _, err := s.client.Update(context.Background(), &statefulSet, metav1.UpdateOptions{}); err != nil {
				global.Logger.Error("Failed to update statefulset", zap.String("statefulset", statefulSet.Name), zap.Error(err))
			} else {
				global.Logger.Info("Successfully updated statefulset", zap.String("statefulset", statefulSet.Name))
			}
			time.Sleep(time.Duration(updateWaitTime) * time.Minute)
		}
	}
	return nil
}

// DaemonSet 更新实现
type daemonSetUpdater struct {
	client appsv1.DaemonSetInterface
}

func (d *daemonSetUpdater) updateResource(updateWaitTime int, resourceType string, excludedImage []string, acrAllImageMap map[string]string) error {
	daemonSets, err := d.client.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, daemonSet := range daemonSets.Items {
		if ok := replaceContainerImages(resourceType, daemonSet.Name, excludedImage, &daemonSet.Spec.Template.Spec.Containers, &daemonSet.Spec.Template.Spec.InitContainers, acrAllImageMap); ok {
			daemonSet.Spec.Template.Spec.ImagePullSecrets = nil
			if _, err := d.client.Update(context.Background(), &daemonSet, metav1.UpdateOptions{}); err != nil {
				global.Logger.Error("Failed to update daemonset", zap.String("daemonset", daemonSet.Name), zap.Error(err))
			} else {
				global.Logger.Info("Successfully updated daemonset", zap.String("daemonset", daemonSet.Name))
			}
			time.Sleep(time.Duration(updateWaitTime) * time.Minute)
		}
	}
	return nil
}

// CronJob 更新实现
type cronJobUpdater struct {
	client batchv1.CronJobInterface
}

func (c *cronJobUpdater) updateResource(updateWaitTime int, resourceType string, excludedImage []string, acrAllImageMap map[string]string) error {
	cronJobs, err := c.client.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, cronJob := range cronJobs.Items {
		// 如果没有需要替换的镜像，跳过当前 cronJob
		if !replaceContainerImages(resourceType, cronJob.Name, excludedImage, &cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers, &cronJob.Spec.JobTemplate.Spec.Template.Spec.InitContainers, acrAllImageMap) {
			continue
		}

		cronJob.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets = nil
		for {
			_, err := c.client.Update(context.Background(), &cronJob, metav1.UpdateOptions{})
			if err != nil {
				if errors.IsConflict(err) {
					// 冲突错误，获取最新版本并重试
					global.Logger.Warn("Conflict detected when updating cronjob, retrying", zap.String("cronjob", cronJob.Name))
					latestCronJob, err := c.client.Get(context.Background(), cronJob.Name, metav1.GetOptions{})
					if err != nil {
						return err
					}
					cronJob = *latestCronJob
					if replaceContainerImages(resourceType, cronJob.Name, excludedImage, &cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers, &cronJob.Spec.JobTemplate.Spec.Template.Spec.InitContainers, acrAllImageMap) {
						cronJob.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets = nil
					}
					continue
				} else {
					global.Logger.Error("Failed to update cronjob", zap.String("cronjob", cronJob.Name), zap.Error(err))
					return err
				}
			} else {
				global.Logger.Info("Successfully updated cronjob", zap.String("cronjob", cronJob.Name))
				break
			}
		}
		time.Sleep(time.Duration(updateWaitTime) * time.Minute)
	}

	return nil
}

type updaterFactory struct {
	clientset *kubernetes.Clientset
}

// 创建不同的资源更新工厂
func (u *updaterFactory) newUpdater(resourceType string, namespace string) resourceUpdater {
	switch resourceType {
	case "deployment":
		return &deploymentUpdater{client: u.clientset.AppsV1().Deployments(namespace)}
	case "statefulset":
		return &statefulSetUpdater{client: u.clientset.AppsV1().StatefulSets(namespace)}
	case "daemonset":
		return &daemonSetUpdater{client: u.clientset.AppsV1().DaemonSets(namespace)}
	case "cronjob":
		return &cronJobUpdater{client: u.clientset.BatchV1().CronJobs(namespace)}
	default:
		return nil
	}
}

// 更改容器镜像
func replaceContainerImages(resourceType, resourceName string, excludedImage []string, containers, initContainers *[]corev1.Container, acrAllImageMap map[string]string) bool {
	updated := false

	// 定义一个通用的处理函数
	processContainers := func(containers *[]corev1.Container) {
		for i := range *containers {
			containerImage := (*containers)[i].Image
			parts := strings.Split(containerImage, "/")
			lastPart := parts[len(parts)-1]

			// 检查是否包含在排除列表中
			if isExcluded(lastPart, excludedImage) {
				global.Logger.Info("Image is in the excluded list", zap.String("resourceType", resourceType), zap.String("resourceName", resourceName), zap.String("image", containerImage))
				continue
			}

			// 检查是否存在于映射表并且是否与目标镜像一致
			if image, ok := acrAllImageMap[lastPart]; ok {
				// 镜像已经一致，跳过更新
				if containerImage == image {
					global.Logger.Info("Image is already up to date", zap.String("resourceType", resourceType), zap.String("resourceName", resourceName), zap.String("image", containerImage))
					continue
				}
				updated = true
				(*containers)[i].Image = image
				global.Logger.Info("Updating image", zap.String("resourceType", resourceType), zap.String("resourceName", resourceName), zap.String("oldImage", containerImage), zap.String("newImage", image))
			} else {
				global.Logger.Warn("Image not found in the ACR image map", zap.String("resourceType", resourceType), zap.String("resourceName", resourceName), zap.String("image", containerImage))
			}
		}
	}

	// 处理普通容器
	processContainers(containers)

	// 处理 init 容器
	if initContainers != nil {
		processContainers(initContainers)
	}

	return updated
}

func isExcluded(image string, excludedImage []string) bool {
	for _, v := range excludedImage {
		if strings.Contains(image, v) {
			return true
		}
	}
	return false
}
