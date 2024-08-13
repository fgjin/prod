package setting

import (
	"github.com/spf13/viper"
)

type Setting struct {
	vp *viper.Viper
}

// 读取配置文件
func NewSetting(filepath string) (*Setting, error) {
	vp := viper.New()
	vp.SetConfigName("config")
	vp.AddConfigPath(filepath)
	vp.SetConfigType("toml")
	// vp.SetConfigFile(filepath)
	if err := vp.ReadInConfig(); err != nil {
		return nil, err
	}
	return &Setting{vp}, nil
}

// 将配置文件反序列化到结构体
func (s *Setting) UnmarshalConfigToStruct(configStruct interface{}) error {
	return s.vp.Unmarshal(configStruct)
}