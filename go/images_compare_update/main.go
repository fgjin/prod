package main

import (
	"images_compare_update/cmd"
	"images_compare_update/global"
	"images_compare_update/internal/get_images"
	"images_compare_update/internal/processing_image_tasks"
	"images_compare_update/internal/update_image_online"

	"images_compare_update/pkg/logger"
	"images_compare_update/pkg/setting"
	"images_compare_update/pkg/utils"
	"log"

	"github.com/joho/godotenv"
)

func init() {
	// 加载 .env 文件中的环境变量
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Failed to load .env file: %v", err)
	}

	// 获取配置文件
	if err := cmd.Execute(); err != nil {
		log.Fatalf("Failed to execute command: %v", err)
	}

	// 初始化配置
	if err := setupSetting(); err != nil {
		log.Fatalf("Failed to setup settings: %v", err)
	}

	// 初始化日志
	if err := setupLogger(); err != nil {
		log.Fatalf("Failed to setup logger: %v", err)
	}
}

func main() {
	// 同步缓冲区的日志到磁盘
	defer global.Logger.Sync()

	// 获取 k8s 和 ACR 的镜像
	k8sImageLastPartSli, k8sImageMap := get_images.GetK8sImages(global.ContextTimeout)
	acrImageSli, _ := get_images.GetAcrImages(global.ConfigStruct.GetAcrPrivateAddr(), global.ConfigStruct.GetInstanceId(),
		global.SemaphoreCap, global.ContextTimeout)

	// 将 ACR 的镜像切片转换为 map
	acrImageMap := utils.ConvertSliceToMap(acrImageSli, func(image string) string { return image }, func(image string) struct{} { return struct{}{} })

	// 查找 ACR 缺失的镜像
	k8sImageInfoSli := utils.GetElementsNotInMap(k8sImageLastPartSli, k8sImageMap, acrImageMap)

	// 从切片中去除从阿里云仓库下载的镜像
	k8sImageInfoSli = utils.RemoveElementsInSlice(k8sImageInfoSli, global.ConfigStruct.GetExcludedDomain())

	// 打印需要上传到 ACR 的镜像
	utils.EchoColor(k8sImageInfoSli)

	// 上传缺失的镜像到 ACR
	processing_image_tasks.ExecImageTasks(global.SemaphoreCap, global.ConfigStruct.GetAcrPrivateAddr(), global.ConfigStruct.GetHabor(),
		global.ConfigStruct.GetInstanceId(), global.ConfigStruct.GetUsername(), k8sImageInfoSli)

	// 全量获取所有 ACR 的镜像
	_, acrAllImageMap := get_images.GetAcrImages(global.ConfigStruct.GetAcrPrivateAddr(), global.ConfigStruct.GetInstanceId(),
		global.SemaphoreCap, global.ContextTimeout)
	// fmt.Println(acrAllImageMap)
	// 更新 k8s 资源相关字段
	update_image_online.UpdateResourceFields(global.ConfigStruct.GetUpdateConcurrency(), global.ConfigStruct.GetUpdateWaitTime(), 
		global.ConfigStruct.GetExcludedImage(), global.Namespace, acrAllImageMap)
}

// 根据配置文件位置读取配置文件
func setupSetting() error {
	setting, err := setting.NewSetting(global.ConfigPath)
	if err != nil {
		return err
	}
	if err := setting.UnmarshalConfigToStruct(&global.ConfigStruct); err != nil {
		return err
	}
	return nil
}

// 初始化日志配置
func setupLogger() error {
	return logger.InitLogger(global.ConfigStruct.GetLogConfig())
}
