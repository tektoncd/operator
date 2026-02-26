# Centralized TLS Configuration Support

This change adds support for centralized TLS configuration from OpenShift's APIServer resource, enabling Tekton components to inherit TLS settings (minimum version, cipher suites, curve preferences) from the cluster-wide security policy.

## Key Changes

### 1. New Configuration Flag

- Added `EnableCentralTLSConfig` boolean field to `TektonConfig.Spec.Platforms.OpenShift`
- When enabled, TLS settings from the cluster's APIServer are automatically injected into supported components
- Default: `false` (opt-in)

### 2. APIServer Watcher

- Single centralized watcher in TektonConfig controller monitors the APIServer cluster resource
- Uses a shared informer with 30-minute resync interval
- When APIServer TLS profile changes, enqueues TektonConfig for reconciliation

### 3. Platform Data Annotation

- TektonConfig OpenShift extension resolves TLS config via `GetPlatformData()` and returns a SHA-256 hash of the TLS profile
- The shared TektonConfig reconciler stamps this as an `operator.tekton.dev/platform-data-hash` annotation on sub-component CRs (e.g., TektonResult)
- When the annotation value changes, the sub-component CR is updated, triggering its reconciler
- The sub-component reconciler includes the annotation value in the installer set hash computation, ensuring the InstallerSet is updated when TLS config changes

### 4. TektonResult Integration

- First component to support centralized TLS configuration
- Injects `TLS_MIN_VERSION`, `TLS_CIPHER_SUITES`, and `TLS_CURVE_PREFERENCES` environment variables into the Results API deployment

## TLS Configuration Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           INITIALIZATION                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  1. TektonConfig Controller starts                                           │
│     └─► setupAPIServerTLSWatch() creates shared informer for APIServer      │
│         └─► Stores lister in occommon.SetSharedAPIServerLister()            │
│         └─► Registers event handler to enqueue TektonConfig on changes      │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                           RECONCILIATION                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  2. TektonConfig reconciliation (shared reconciler)                          │
│     ├─► TektonConfig extension.GetPlatformData()                            │
│     │   └─► Resolves TLS profile from shared APIServer lister               │
│     │   └─► Returns SHA-256 hash of TLS profile                             │
│     ├─► Stamps fingerprint as annotation on TektonResult CR                 │
│     │   (operator.tekton.dev/platform-data-hash)                                  │
│     └─► UpdateResult detects annotation change → updates CR                 │
│                                                                              │
│  3. TektonResult reconciliation triggered by CR update                       │
│     │                                                                        │
│     ▼                                                                        │
│  4. Extension.PreReconcile(ctx) called                                       │
│     └─► ResolveCentralTLSToEnvVars(ctx)                                     │
│         ├─► Check TektonConfig.Spec.Platforms.OpenShift.EnableCentralTLSConfig│
│         │   └─► If false, return nil (no central TLS)                       │
│         └─► GetTLSProfileFromAPIServer → TLSEnvVarsFromProfile              │
│             └─► Store result in oe.resolvedTLSConfig                        │
│                                                                              │
│  5. Hash computation includes platform-data annotation value                 │
│     └─► Change in TLS config → different hash → installer set update        │
│                                                                              │
│  6. Extension.Transformers() called                                          │
│     └─► If resolvedTLSConfig != nil:                                        │
│         └─► InjectTLSEnvVars() transformer added                            │
│                                                                              │
│  7. Manifests transformed                                                    │
│     └─► InjectTLSEnvVars() adds env vars to Results API deployment:         │
│         ├─► TLS_MIN_VERSION                                                 │
│         ├─► TLS_CIPHER_SUITES                                               │
│         └─► TLS_CURVE_PREFERENCES                                           │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                           AUTOMATIC UPDATES                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  8. When APIServer TLS profile changes:                                      │
│     └─► Informer event handler triggers                                     │
│         └─► Enqueues TektonConfig for reconciliation                        │
│             └─► GetPlatformData() returns new TLS fingerprint               │
│                 └─► Annotation on TektonResult CR updated                   │
│                     └─► TektonResult reconciler triggered                   │
│                         └─► New hash → InstallerSet updated                 │
│                             └─► Deployment updated with new env vars        │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Known Limitations

- **Default TLS profile when none is explicitly set**: When `enableCentralTLSConfig` is enabled
  but no explicit `.spec.tlsSecurityProfile` is configured on the APIServer resource, the operator
  injects the Intermediate profile values (TLS 1.2, standard cipher suite). This is because
  library-go's `ObserveTLSSecurityProfile` defaults to the Intermediate profile when the field is
  nil, which is the same behavior used by other OpenShift components.

- **Curve preferences**: `TLS_CURVE_PREFERENCES` is currently not populated and defaults to
  Go's standard library values. This will be addressed once [openshift/api#2583](https://github.com/openshift/api/pull/2583)
  is merged and the OpenShift API exposes curve preference configuration.
