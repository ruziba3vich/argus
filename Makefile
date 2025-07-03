migrate-create:  ### create new migration
	migrate create -ext sql -dir migrations '$(word 2,$(MAKECMDGOALS))'

swag: ### swag init
	swag init -g api/router.go
