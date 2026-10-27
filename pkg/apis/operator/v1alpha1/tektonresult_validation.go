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

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"knative.dev/pkg/apis"
)

const (
	LogsTypeLoki = "loki"
)

func (tp *TektonResult) Validate(ctx context.Context) (errs *apis.FieldError) {

	if apis.IsInDelete(ctx) {
		return nil
	}

	if tp.GetName() != ResultResourceName {
		errMsg := fmt.Sprintf("metadata.name, Only one instance of TektonResult is allowed by name, %s", ResultResourceName)
		return errs.Also(apis.ErrInvalidValue(tp.GetName(), errMsg))
	}
	errs = errs.Also(tp.Spec.validate("spec"))
	return errs
}

func (trs *TektonResultSpec) validate(path string) (errs *apis.FieldError) {
	// validate the embedded CommonSpec (e.g. targetNamespace denylist),
	// which TektonResult would otherwise bypass since it does not call
	// CommonSpec.validate the way the other components do.
	errs = errs.Also(trs.CommonSpec.validate(path))

	if trs.LokiStackName != "" {
		if strings.ToLower(trs.LogsType) != LogsTypeLoki && trs.LogsType != "" {
			errMsg := fmt.Sprintf("Loki stack is only supported when logs_type is loki or empty, got logs_type: %s", trs.LogsType)
			errs = errs.Also(apis.ErrInvalidValue(trs.LogsType, fmt.Sprintf("%s.logs_type", path), errMsg))
		}
		if trs.LokiStackNamespace == "" {
			errMsg := "Loki stack namespace is required when loki_stack_name is provided"
			errs = errs.Also(apis.ErrInvalidValue(trs.LokiStackNamespace, fmt.Sprintf("%s.loki_stack_namespace", path), errMsg))
		}
	}

	// validate Results API settings, including route TLS termination
	errs = errs.Also(trs.ResultsAPIProperties.validate(path))

	// validate performance properties
	errs = errs.Also(trs.Performance.Validate(fmt.Sprintf("%s.performance", path)))

	// validate watcher properties
	errs = errs.Also(trs.Watcher.Validate(fmt.Sprintf("%s.watcher", path)))

	errs = errs.Also(trs.NetworkPolicy.validate(fmt.Sprintf("%s.networkPolicy", path)))

	return errs
}

// validate checks Results API fields that have a fixed set of values or a
// numeric range.
func (props ResultsAPIProperties) validate(path string) (errs *apis.FieldError) {
	errs = errs.Also(props.validateRouteTLSTermination(path))

	if props.SQLLogLevel != "" {
		switch props.SQLLogLevel {
		case "silent", "error", "warn", "info":
		default:
			errs = errs.Also(apis.ErrInvalidValue(props.SQLLogLevel, fmt.Sprintf("%s.sql_log_level", path), "must be one of: silent, error, warn, info"))
		}
	}

	if props.TLSMinVersion != "" {
		switch props.TLSMinVersion {
		case "1.0", "1.1", "1.2", "1.3", "TLS1.0", "TLS1.1", "TLS1.2", "TLS1.3", "TLSv1.0", "TLSv1.1", "TLSv1.2", "TLSv1.3":
		default:
			errs = errs.Also(apis.ErrInvalidValue(props.TLSMinVersion, fmt.Sprintf("%s.tls_min_version", path), "must be one of: 1.0, 1.1, 1.2, 1.3"))
		}
	}

	errs = errs.Also(validatePort(props.ProfilingPort, fmt.Sprintf("%s.profiling_port", path)))
	errs = errs.Also(validateNonNegativeInt(props.DBMaxIdleConnections, fmt.Sprintf("%s.db_max_idle_connections", path)))
	errs = errs.Also(validateNonNegativeInt(props.DBMaxOpenConnections, fmt.Sprintf("%s.db_max_open_connections", path)))
	errs = errs.Also(validateNonNegativeInt(props.GRPCWorkerPool, fmt.Sprintf("%s.grpc_worker_pool", path)))
	errs = errs.Also(validateNonNegativeInt(props.ConverterDBLimit, fmt.Sprintf("%s.converter_db_limit", path)))
	return errs
}

func validateNonNegativeInt(value *int64, field string) *apis.FieldError {
	if value == nil || *value >= 0 {
		return nil
	}
	return apis.ErrInvalidValue(*value, field, "must be greater than or equal to 0")
}

func validatePort(value *int64, field string) *apis.FieldError {
	if value == nil || (*value >= 1 && *value <= 65535) {
		return nil
	}
	return apis.ErrInvalidValue(*value, field, "must be between 1 and 65535")
}

// validateRouteTLSTermination validates that the route TLS termination type is one of the
// supported values. It is called from both TektonResult and TektonConfig validators so that
// unsupported values are rejected at the parent (TektonConfig) level as well.
func (props ResultsAPIProperties) validateRouteTLSTermination(path string) *apis.FieldError {
	if props.RouteTLSTermination == "" {
		return nil
	}
	switch props.RouteTLSTermination {
	case "edge", "reencrypt", "passthrough":
		return nil
	default:
		errMsg := fmt.Sprintf("unsupported route TLS termination type %q, must be one of: edge, reencrypt, passthrough", props.RouteTLSTermination)
		return apis.ErrInvalidValue(props.RouteTLSTermination, fmt.Sprintf("%s.route_tls_termination", path), errMsg)
	}
}
