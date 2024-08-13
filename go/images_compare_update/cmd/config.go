package cmd

import (
	"images_compare_update/global"
	"strings"

	"github.com/spf13/cobra"
)

var (
	configDir  string
	namespaces []string
)

// 定义命令行的主要参数
var configCmd = &cobra.Command{
	Use: "config",
	// 简短帮助说明
	Short: "指定配置路径",
	// 详细帮助说明
	Long: configDesc,
	Run: func(cmd *cobra.Command, args []string) {
		// 设置全局变量
		global.ConfigPath = configDir
		global.Namespace = namespaces
	},
}

var configDesc = strings.Join([]string{
	"该子命令用于指定配置文件路径，流程如下：",
	"1: 指定配置文件文件夹即可",
	"2: 指定配置名称必须为config.toml",
}, "\n")

func init() {
	// 绑定命令行输入，绑定一个参数
	// 参数分别表示，绑定的变量，参数长名(--str)，参数短名(-s)，默认内容，帮助信息
	configCmd.Flags().StringVarP(&configDir, "conf", "c", "config", "请选择配置文件")
	configCmd.Flags().StringSliceVarP(&namespaces, "namespace", "n", []string{}, "请选择一个或多个命名空间")
	configCmd.MarkFlagRequired("namespaces")
}
