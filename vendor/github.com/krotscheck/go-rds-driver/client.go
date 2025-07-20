package rds

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
)

// AWSClientInterface interface that captures methods required by the driver. In this case, replicating the RDS API
type AWSClientInterface interface {
	ExecuteStatement(ctx context.Context, e *rdsdata.ExecuteStatementInput, optFns ...func(*rdsdata.Options)) (*rdsdata.ExecuteStatementOutput, error)
	BeginTransaction(ctx context.Context, b *rdsdata.BeginTransactionInput, optFns ...func(*rdsdata.Options)) (*rdsdata.BeginTransactionOutput, error)
	CommitTransaction(ctx context.Context, c *rdsdata.CommitTransactionInput, optFns ...func(*rdsdata.Options)) (*rdsdata.CommitTransactionOutput, error)
	RollbackTransaction(ctx context.Context, r *rdsdata.RollbackTransactionInput, optFns ...func(*rdsdata.Options)) (*rdsdata.RollbackTransactionOutput, error)
}
