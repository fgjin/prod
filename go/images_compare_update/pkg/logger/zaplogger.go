package logger

import (
	"images_compare_update/global"
	"images_compare_update/pkg/setting"
	"os"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// 初始化Logger
func InitLogger(cfg *setting.LogConf) (err error) {
	// 获取日志写入器
	writeSyncer := getLogWriter(cfg.LogFile, cfg.MaxSize, cfg.MaxBackups, cfg.MaxAge)
	// 获取日志编码器
	encoder := getEncoder(cfg.Env)
	// 设置日志级别
	level := new(zapcore.Level)
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		return err
	}
	// 同时将日志写入文件和标准输出
	multiSyncer := zapcore.NewMultiWriteSyncer(writeSyncer, zapcore.AddSync(os.Stdout))
	core := zapcore.NewCore(encoder, multiSyncer, level)
	// 全局日志记录器
	global.Logger = zap.New(core, zap.AddCaller())

	return nil
}

// 定义日志编码格式
func getEncoder(env string) zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	// encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006/01/02 15:04:05")
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	if env == "test" {
		// 使用带颜色的日志级别
		encoderConfig.EncodeLevel = zapcore.LowercaseColorLevelEncoder
		return zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 默认情况下返回 JSON 编码器
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

/*
定义日志文件配置
filename: 日志文件名
maxSize: 日志文件最大大小（以MB为单位）
maxBackup: 日志文件最多保留的备份数
maxAge: 日志文件最多保留的天数
*/
func getLogWriter(filename string, maxSize, maxBackup, maxAge int) zapcore.WriteSyncer {
	return zapcore.AddSync(&lumberjack.Logger{
		Filename:   filename,
		MaxSize:    maxSize,
		MaxBackups: maxBackup,
		MaxAge:     maxAge,
	})
}
