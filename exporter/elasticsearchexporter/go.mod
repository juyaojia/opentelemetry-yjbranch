module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter

go 1.17

require (
	github.com/cenkalti/backoff/v4 v4.1.2
	github.com/elastic/go-elasticsearch/v7 v7.17.1
	github.com/elastic/go-structform v0.0.9
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.47.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.47.1-0.20220316214317-e6ea17cf27a8
	go.opentelemetry.io/collector/model v0.47.1-0.20220316214317-e6ea17cf27a8
	go.uber.org/multierr v1.8.0
	go.uber.org/zap v1.21.0

)

require (
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/knadh/koanf v1.4.0 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/otel v1.4.1 // indirect
	go.opentelemetry.io/otel/metric v0.27.0 // indirect
	go.opentelemetry.io/otel/trace v1.4.1 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common