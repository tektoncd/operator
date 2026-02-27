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

### 3. Extension Interface Enhancement

- Added `GetHashData() string` method to the Extension interface
- Enables components to include platform-specific data in installer set hash computation
- Triggers installer set updates when TLS configuration changes

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
│  2. TektonResult reconciliation triggered                                    │
│     │                                                                        │
│     ▼                                                                        │
│  3. Extension.PreReconcile(ctx) called                                       │
│     │                                                                        │
│     ├─► resolveTLSConfig(ctx)                                               │
│     │   ├─► Check TektonConfig.Spec.Platforms.OpenShift.EnableCentralTLSConfig│
│     │   │   └─► If false, return nil (no central TLS)                       │
│     │   │                                                                    │
│     │   └─► occommon.GetTLSEnvVarsFromAPIServer(ctx)                        │
│     │       ├─► Read from shared APIServer lister (no API call)             │
│     │       ├─► Use library-go's ObserveTLSSecurityProfile()                │
│     │       └─► Return TLSEnvVars{MinVersion, CipherSuites, CurvePreferences}│
│     │                                                                        │
│     └─► Store result in oe.resolvedTLSConfig                                │
│         └─► Log: "Injecting central TLS config: MinVersion=..."             │
│                                                                              │
│  4. Hash computation includes Extension.GetHashData()                        │
│     └─► Returns fingerprint: "MinVersion:CipherSuites:CurvePreferences"     │
│     └─► Change in TLS config → different hash → installer set update        │
│                                                                              │
│  5. Extension.Transformers() called                                          │
│     └─► If resolvedTLSConfig != nil:                                        │
│         └─► Add injectTLSConfig() transformer                               │
│                                                                              │
│  6. Manifests transformed                                                    │
│     └─► injectTLSConfig() adds env vars to Results API deployment:          │
│         ├─► TLS_MIN_VERSION                                                 │
│         ├─► TLS_CIPHER_SUITES                                               │
│         └─► TLS_CURVE_PREFERENCES                                           │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────────────┐
│                           AUTOMATIC UPDATES                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  7. When APIServer TLS profile changes:                                      │
│     └─► Informer event handler triggers                                     │
│         └─► Enqueues TektonConfig for reconciliation                        │
│             └─► TektonResult reconciled with new TLS config                 │
│                 └─► New hash computed → InstallerSet updated                │
│                     └─► Deployment updated with new env vars                │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```
