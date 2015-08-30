
all: test-circleci

test-circleci:
	${GODEP} go test ./... -logtostderr -check.vv -v ${TEST_ARGS}
