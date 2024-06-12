tidy:
	GOPROXY=direct go get -u -v all 
	@go mod tidy --compat=1.22
