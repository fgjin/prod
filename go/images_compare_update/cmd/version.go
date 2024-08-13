package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/q191201771/naza/pkg/bininfo"
	"github.com/spf13/cobra"
)

// 定义子命令行的主要参数
var versionCmd = &cobra.Command{
	// 子命令的标识
	Use: "version",
	// 简短帮助说明
	Short: "获取当前版本",
	// 详细帮助说明
	Long: versionDesc,
	Run: func(cmd *cobra.Command, args []string) {
		_, _ = fmt.Fprint(os.Stderr, bininfo.StringifyMultiLine())
		os.Exit(0)
	},
}

var versionDesc = strings.Join([]string{
	"该子命令支持获取当前版本信息:",
	"GitTag:        tag号",
	"GitCommitLog:  当前commit日志",
	"GitStatus:     status状态",
	"BuildTime:     构建时间",
	"GoVersion:     golang版本",
	"runtime:       运行环境",
}, "\n")
