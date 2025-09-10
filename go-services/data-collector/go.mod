module data-collector

go 1.23

require (
	github.com/aws/aws-lambda-go v1.47.0
	github.com/aws/aws-sdk-go v1.55.5
	gopkg.in/yaml.v3 v3.0.1
	shared/logger v0.0.0
)

require github.com/jmespath/go-jmespath v0.4.0 // indirect

replace shared/logger => ../shared/logger
