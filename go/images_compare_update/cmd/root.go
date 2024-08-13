package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{}

// 执行命令行指令
func Execute() error {
	// 检查是否需要显示根命令的帮助信息
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		rootCmd.Help()
		return errors.New("only output root help information")
	}

	// 检查是否需要显示子命令的帮助信息
	if len(os.Args) > 2 && (os.Args[2] == "-h" || os.Args[2] == "--help") {
		subCommand := os.Args[1]
		for _, cmd := range rootCmd.Commands() {
			if cmd.Use == subCommand {
				cmd.Help()
				return fmt.Errorf("only output %s help information", subCommand)
			}
		}
	}

	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}
