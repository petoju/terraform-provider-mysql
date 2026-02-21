
AWS_ACCOUNT_ID = $(shell aws sts get-caller-identity | jq -r .Account )
export AWS_REGION=us-west-2
export RDS_MYSQL_DB_NAME=$(shell more ./terraform/terraform.tfstate | jq -r .outputs.mysql_database_name.value)
export RDS_MYSQL_ARN=$(shell more ./terraform/terraform.tfstate | jq -r .outputs.mysql_resource_arn.value)
export RDS_POSTGRES_DB_NAME=$(shell more ./terraform/terraform.tfstate | jq -r .outputs.postgresql_database_name.value)
export RDS_POSTGRES_ARN=$(shell more ./terraform/terraform.tfstate | jq -r .outputs.postgresql_resource_arn.value)
export RDS_SECRET_ARN=$(shell more ./terraform/terraform.tfstate | jq -r .outputs.rds_secret_arn.value)

.PHONY: env
env:
	env | grep RDS

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: clean
clean:
	rm -rf ./reports || true
	rm ./client_mocks_test.go || true

reports:
	mkdir -p ./reports

client_mocks_test.go:
	go generate ./...

.PHONY: test
test: reports/coverage.xml reports/html/index.html

.PHONY: vet
vet: reports
	go vet ./...

.PHONY: lint
lint: reports
	go tool golang.org/x/lint/golint ./...

.PHONY: gocritic
gocritic: reports
	go tool github.com/go-critic/go-critic/cmd/gocritic check ./...

.PHONY: sec
sec: reports
	gosec ./...


.PHONY: vulncheck
vulncheck:
	go tool golang.org/x/vuln/cmd/govulncheck ./...

reports/coverage.out: reports client_mocks_test.go
	go tool gotest.tools/gotestsum --junitfile reports/unit-tests.xml -- -p 1 -covermode=atomic -coverpkg=./... -coverprofile=reports/coverage.out ./...
	go tool github.com/axw/gocov/gocov convert reports/coverage.out | go tool github.com/axw/gocov/gocov report

reports/html/index.html: reports/coverage.out
	mkdir -p ./reports/html
	go tool github.com/axw/gocov/gocov convert ./reports/coverage.out | go tool github.com/matm/gocov-html/cmd/gocov-html > ./reports/html/index.html

reports/coverage.xml: reports/coverage.out
	go tool github.com/axw/gocov/gocov convert ./reports/coverage.out | go tool github.com/AlekSi/gocov-xml > ./reports/coverage.xml

.PHONY: checks
checks: test vet lint gocritic sec vulncheck

.PHONY: update
update:
	go get -t -u ./...
	go mod tidy -compat=1.25
