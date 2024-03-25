package operator

//go:generate mockgen -destination=./mocks/rpc_client.go -package=mocks avs-oracle/operator AggregatorRpcClienter
