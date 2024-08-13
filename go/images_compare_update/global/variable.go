// 定义全局变量
package global

import (
	"images_compare_update/pkg/setting"

	"go.uber.org/zap"
)

var (
	ConfigStruct *setting.Config
	Logger       *zap.Logger
	ConfigPath   = "config"
	Namespace    []string
)

const (
	SemaphoreCap   = 10
	ContextTimeout = 120
)
