/*
Copyright 2024 The Tekton Authors

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

package secerrors

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogError logs an error with appropriate security considerations
// It logs the sanitized message at the specified level and the internal error at debug level
func LogError(logger *zap.SugaredLogger, err error, msg string, keysAndValues ...interface{}) {
	if err == nil {
		return
	}
	
	// Get the safe error message
	safeMsg := SafeErrorMessage(err)
	
	// Combine the message and safe error
	fullMsg := msg
	if safeMsg != "" {
		fullMsg = fullMsg + ": " + safeMsg
	}
	
	// Log the safe message at error level
	logger.Errorw(fullMsg, keysAndValues...)
	
	// If it's a SecureError, log the internal error at debug level for troubleshooting
	var secErr *SecureError
	if logger.Desugar().Core().Enabled(zapcore.DebugLevel) {
		if AsSecureError(err, &secErr) && secErr.InternalError() != nil {
			logger.Debugw("internal error details", "internal_error", secErr.InternalError().Error())
		}
	}
}

// LogErrorf is like LogError but with formatted message
func LogErrorf(logger *zap.SugaredLogger, err error, format string, args ...interface{}) {
	if err == nil {
		return
	}
	
	// Get the safe error message
	safeMsg := SafeErrorMessage(err)
	
	// Log the safe message
	logger.Errorf(format+": %s", append(args, safeMsg)...)
	
	// Log internal details at debug level
	var secErr *SecureError
	if logger.Desugar().Core().Enabled(zapcore.DebugLevel) {
		if AsSecureError(err, &secErr) && secErr.InternalError() != nil {
			logger.Debugf("internal error details: %v", secErr.InternalError())
		}
	}
}

// LogWarn logs a warning with security sanitization
func LogWarn(logger *zap.SugaredLogger, err error, msg string, keysAndValues ...interface{}) {
	if err == nil {
		return
	}
	
	safeMsg := SafeErrorMessage(err)
	fullMsg := msg
	if safeMsg != "" {
		fullMsg = fullMsg + ": " + safeMsg
	}
	
	logger.Warnw(fullMsg, keysAndValues...)
}

// LogWarnf is like LogWarn but with formatted message
func LogWarnf(logger *zap.SugaredLogger, err error, format string, args ...interface{}) {
	if err == nil {
		return
	}
	
	safeMsg := SafeErrorMessage(err)
	logger.Warnf(format+": %s", append(args, safeMsg)...)
}

// SafeZapError creates a zap field with sanitized error message
func SafeZapError(err error) zap.Field {
	if err == nil {
		return zap.Skip()
	}
	return zap.String("error", SafeErrorMessage(err))
}

// InternalZapError creates a zap field with the internal error (for debug logging only)
// This should only be used when debug logging is enabled
func InternalZapError(err error) zap.Field {
	var secErr *SecureError
	if AsSecureError(err, &secErr) && secErr.InternalError() != nil {
		return zap.String("internal_error", secErr.InternalError().Error())
	}
	return zap.Skip()
}

// AsSecureError is a helper that wraps errors.As for SecureError
func AsSecureError(err error, target **SecureError) bool {
	if err == nil {
		return false
	}
	
	switch e := err.(type) {
	case *SecureError:
		*target = e
		return true
	default:
		return false
	}
}

// SecureLogFields creates zap fields with proper sanitization
// It returns both a safe field for general logging and an internal field for debug logging
func SecureLogFields(err error) []zap.Field {
	if err == nil {
		return []zap.Field{}
	}
	
	fields := []zap.Field{SafeZapError(err)}
	
	// Add internal error field if it's a SecureError
	var secErr *SecureError
	if AsSecureError(err, &secErr) && secErr.InternalError() != nil {
		fields = append(fields, zap.String("error_category", string(secErr.Category())))
	}
	
	return fields
}

