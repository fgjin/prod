package utils

import (
	"fmt"
	"images_compare_update/global"
	"sort"
	"strings"

	"github.com/fatih/color"
	"go.uber.org/zap"
)

// 切片去重
func RemoveSliceDuplicates(slice []string) []string {
	uniqueMap := make(map[string]bool)
	result := make([]string, 0)
	for _, s := range slice {
		if !uniqueMap[s] {
			uniqueMap[s] = true
			result = append(result, s)
		}
	}
	return result
}

// 泛型函数，将切片转换成 map
func ConvertSliceToMap[T any, K comparable, V any](slice []T, keyFunc func(T) K, valueFunc func(T) V) map[K]V {
	result := make(map[K]V)
	for _, item := range slice {
		key := keyFunc(item)
		value := valueFunc(item)
		result[key] = value
	}
	global.Logger.Debug("Converted slice to map", zap.Any("result", result), zap.Int("length", len(result)))
	return result
}

// 返回在切片中但不在 map 中的元素
func GetElementsNotInMap(slice []string, map1 map[string]string, map2 map[string]struct{}) []string {
	var result []string
	for _, s := range slice {
		if _, exists := map2[s]; !exists {
			if v, found := map1[s]; found {
				result = append(result, v)
			}
		}
	}
	sort.Strings(result)
	global.Logger.Debug("Filtered elements not in map", zap.Any("result", result), zap.Int("length", len(result)))
	return result
}

// 去除切片中含有特定字符的元素
func RemoveElementsInSlice(slice, substringsToRemove []string) []string {
	var result []string
	for _, item := range slice {
		shouldRemove := false
		for _, substr := range substringsToRemove {
			if strings.Contains(item, substr) {
				shouldRemove = true
				break
			}
		}
		if !shouldRemove {
			result = append(result, item)
		}
	}
	global.Logger.Debug("Removed elements containing specific substrings", zap.Any("result", result), zap.Int("length", len(result)))
	return result
}

// 输出颜色字体
func EchoColor(slice []string) {
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	header := fmt.Sprintf("%s %s %s %s", green("####################"), yellow("需要同步的镜像，数量:"), yellow(len(slice)), green("####################"))

	fmt.Println(header)
	for k, v := range slice {
		fmt.Println(k+1, v)
	}
	// fmt.Println(green("##################################"))
	fmt.Println()
}
