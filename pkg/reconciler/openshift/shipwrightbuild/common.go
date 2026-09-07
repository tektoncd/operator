package shipwrightbuild

const (
	// tlsMinVersionFlag and tlsCipherSuites are the webhook command-line flags used
	// to pin the minimum TLS version and allowed cipher suites to the cluster-wide
	// TLS policy resolved from TektonConfig.
	tlsMinVersionFlag = "--tls-min-version"
	tlsCipherSuites   = "--tls-cipher-suites"
)
