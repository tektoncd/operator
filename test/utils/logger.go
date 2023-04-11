/*
Copyright 2023 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// logger for e2e tests
func Logger() *zap.SugaredLogger {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.TimeKey = "time"
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.NameKey = "logger"
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.MessageKey = "msg"
	config.EncoderConfig.StacktraceKey = "stacktrace"
	config.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	newLevel, err := zapcore.ParseLevel(GetEnvironment(ENV_LOG_LEVEL, "debug"))
	if err != nil {
		newLevel = zapcore.InfoLevel
	}
	config.Level = zap.NewAtomicLevelAt(newLevel)

	logger, err := config.Build()
	if err != nil {
		return zap.NewNop().Sugar()
	}

	// do not print stacktrace on warn level
	logger = logger.WithOptions(zap.AddStacktrace(zap.ErrorLevel))
	return logger.Sugar()
}
