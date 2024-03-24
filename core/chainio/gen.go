package chainio

//go:generate mockgen -destination=./mocks/avs_subscriber.go -package=mocks avs-oracle/core/chainio AvsSubscriberer
//go:generate mockgen -destination=./mocks/avs_writer.go -package=mocks avs-oracle/core/chainio AvsWriterer
//go:generate mockgen -destination=./mocks/avs_reader.go -package=mocks avs-oracle/core/chainio AvsReaderer
