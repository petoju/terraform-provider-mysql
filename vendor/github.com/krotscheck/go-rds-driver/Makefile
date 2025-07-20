
AWS_ACCOUNT_ID = $(shell aws sts get-caller-identity | jq -r .Account )
export AWS_REGION=us-west-2
export RDS_MYSQL_DB_NAME=$(shell more ./terraform/terraform.tfstate | jq -r .outputs.mysql_database_name.value)
export RDS_MYSQL_ARN=$(shell more ./terraform/terraform.tfstate | jq -r .outputs.mysql_resource_arn.value)
export RDS_POSTGRES_DB_NAME=$(shell more ./terraform/terraform.tfstate | jq -r .outputs.postgresql_database_name.value)
export RDS_POSTGRES_ARN=$(shell more ./terraform/terraform.tfstate | jq -r .outputs.postgresql_resource_arn.value)
export RDS_SECRET_ARN=$(shell more ./terraform/terraform.tfstate | jq -r .outputs.rds_secret_arn.value)

.PHONY: fmt test vet lint sec clean

fmt:
	go fmt ./...

clean:
	rm -rf ./reports || true
	rm ./client_mocks_test.go || true

reports:
	mkdir -p ./reports

client_mocks_test.go:
	go generate ./...

test_deps:
	go install gotest.tools/gotestsum@v1.11.0
	go install github.com/golang/mock/mockgen@v1.6.0
	go install github.com/axw/gocov/gocov@v1.1.0
	go install github.com/AlekSi/gocov-xml@v1.1.0
	go install github.com/matm/gocov-html/cmd/gocov-html@v1.4.0

test: test_deps reports/coverage.xml reports/html/index.html

vet: reports
	go vet ./...

lint: reports
	golint ./...

tidy:
	go mod tidy -compat=1.22

sec: reports
	gosec ./...

reports/coverage.out: reports client_mocks_test.go
	gotestsum --junitfile reports/unit-tests.xml -- -p 1 -covermode=atomic -coverpkg=./... -coverprofile=reports/coverage.out ./...
	go tool cover -func reports/coverage.out

reports/html/index.html: reports/coverage.out
	mkdir -p ./reports/html
	gocov convert ./reports/coverage.out | gocov-html > ./reports/html/index.html

reports/coverage.xml: reports/coverage.out
	gocov convert ./reports/coverage.out | gocov-xml > ./reports/coverage.xml

checks: test vet lint sec

update:
	go get -t -u ./...
