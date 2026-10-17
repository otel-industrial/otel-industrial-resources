// Module tools pins the code generators used by the receivers, so every
// contributor and CI run the same versions. Run them with
// `go tool -modfile=<path to this go.mod> <tool>`; the receiver Makefiles
// wrap this as `make generate`.
//
// mdatagen is pinned here instead of via `go install ...@version` because its
// release go.mod files contain local replace directives, which `go install`
// rejects. Replace directives in a dependency are ignored, so it builds here.
module github.com/otel-industrial/otel-industrial-resources/otel-industrial-collector/internal/tools

go 1.25.0

tool go.opentelemetry.io/collector/cmd/mdatagen

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	go.opentelemetry.io/collector/cmd/mdatagen v0.152.0 // indirect
	go.opentelemetry.io/collector/component v1.58.0 // indirect
	go.opentelemetry.io/collector/confmap v1.58.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.58.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.58.0 // indirect
	go.opentelemetry.io/collector/filter v0.152.0 // indirect
	go.opentelemetry.io/collector/internal/schemagen v0.152.0 // indirect
	go.opentelemetry.io/collector/pdata v1.58.0 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/mod v0.35.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	golang.org/x/tools v0.44.0 // indirect
)
