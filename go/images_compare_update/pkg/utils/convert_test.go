package utils

import (
	"fmt"
	"images_compare_update/global"
	"reflect"
	"testing"

	"go.uber.org/zap"
)

// 初始化测试环境
func init() {
	// global.Logger = zap.NewNop() // 不输出日志
	global.Logger, _ = zap.NewDevelopment() // 输出日志
}

// 测试切片去重
func TestRemoveSliceDuplicates(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{name: "Test samples 01", input: []string{"a", "b", "a", "c", "b"}, expected: []string{"a", "b", "c"}},
		{name: "Test samples 02", input: []string{"a", "a", "a"}, expected: []string{"a"}},
		{name: "Test samples 03", input: []string{}, expected: []string{}},
	}
	t.Parallel()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := RemoveSliceDuplicates(test.input)
			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("RemoveSliceDuplicates(%v) = %v; want %v", test.input, result, test.expected)
			}
		})
	}
}

// 测试切片转换成 map
func TestConvertSliceToMap(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]struct{}
	}{
		{name: "Test samples 01", input: []string{"golang", "python"}, expected: map[string]struct{}{"golang": {}, "python": {}}},
	}
	t.Parallel()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := ConvertSliceToMap(test.input, func(s string) string { return s }, func(s string) struct{} { return struct{}{} })
			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("ConvertSliceToMap(%v) = %v; want %v", test.input, result, test.expected)
			}
		})
	}
}

// 测试返回在切片中但不在 map 中的元素
func TestGetElementsNotInMap(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		map1     map[string]string
		map2     map[string]struct{}
		expected []string
	}{
		{
			name:     "Test samples 01",
			slice:    []string{"golang", "python"},
			map1:     map[string]string{"golang": "domain/ns/golang", "python": "domain/ns/python"},
			map2:     map[string]struct{}{"golang": {}},
			expected: []string{"domain/ns/python"},
		},
	}
	t.Parallel()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fmt.Printf("Slice: %v\n", test.slice)
			fmt.Printf("Map1: %v\n", test.map1)
			fmt.Printf("Map2: %v\n", test.map2)
			fmt.Printf("Expected: %v\n", test.expected)
			result := GetElementsNotInMap(test.slice, test.map1, test.map2)
			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("GetElementsNotInMap(%v, %v, %v) = %v; want %v", test.slice, test.map1, test.map2, result, test.expected)
			}
		})
	}
}

// 测试去除切片中含有特定字符的元素
func TestRemoveElementsInSlice(t *testing.T) {
	tests := []struct {
		name               string
		instanceSli        []string
		substringsToRemove []string
		expected           []string
	}{
		{name: "Test samples 01", instanceSli: []string{"golang", "python", "shell"}, substringsToRemove: []string{"go"}, expected: []string{"python", "shell"}},
		{name: "Test samples 02", instanceSli: []string{"golang", "python", "shell"}, substringsToRemove: []string{"go", "py"}, expected: []string{"shell"}},
		{name: "Test samples 03", instanceSli: []string{"golang", "python", "shell"}, substringsToRemove: []string{"ja"}, expected: []string{"golang", "python", "shell"}},
	}
	t.Parallel()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := RemoveElementsInSlice(test.instanceSli, test.substringsToRemove)
			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("RemoveElementsInSlice(%v, %v) = %v; want %v", test.instanceSli, test.substringsToRemove, result, test.expected)
			}
		})
	}
}
