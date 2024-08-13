package setting

// 定义配置项
type Config struct {
	LogConfig   *LogConf   `toml:"logconfig"`
	AcrConfig   *AcrConf   `toml:"acrconfig"`
	ExtraConfig *ExtraConf `toml:"extraConfig"`
}

// 日志配置项
type LogConf struct {
	Level      string `toml:"level"`
	LogFile    string `toml:"logfile"`
	MaxSize    int    `toml:"maxsize"`
	MaxAge     int    `toml:"maxage"`
	MaxBackups int    `toml:"maxbackups"`
	Env        string `toml:"env"`
}

// ACR 配置项
type AcrConf struct {
	RegionId       string `toml:"regionid"`
	InstanceId     string `toml:"instanceid"`
	Registry       string `toml:"registry"`
	AcrPrivateAddr string `toml:"acrPrivateAddr"`
	AcrPublicAddr  string `toml:"acrPublicAddr"`
	Username       string `toml:"username"`
}

// 额外配置项
type ExtraConf struct {
	Harbor            string   `toml:"harbor"`
	ExcludedDomain    []string `toml:"excludedDomain"`
	ExcludedImage     []string `toml:"excludedImage"`
	UpdateWaitTime    int      `toml:"updateWaitTime"`
	UpdateConcurrency int      `toml:"updateConcurrency"`
}

func (c *Config) GetRegionId() string {
	return c.AcrConfig.RegionId
}

func (c *Config) GetInstanceId() string {
	return c.AcrConfig.InstanceId
}

func (c *Config) GetAcrPrivateAddr() string {
	return c.AcrConfig.AcrPrivateAddr
}

func (c *Config) GetAcrPublicAddr() string {
	return c.AcrConfig.AcrPublicAddr
}

func (c *Config) GetUsername() string {
	return c.AcrConfig.Username
}

func (c *Config) GetHabor() string {
	return c.ExtraConfig.Harbor
}

func (c *Config) GetExcludedDomain() []string {
	return c.ExtraConfig.ExcludedDomain
}

func (c *Config) GetExcludedImage() []string {
	return c.ExtraConfig.ExcludedImage
}

func (c *Config) GetUpdateWaitTime() int {
	return c.ExtraConfig.UpdateWaitTime
}

func (c *Config) GetUpdateConcurrency() int {
	return c.ExtraConfig.UpdateConcurrency
}

func (c *Config) GetLogConfig() *LogConf {
	return c.LogConfig
}
