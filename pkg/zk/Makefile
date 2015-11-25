
all: test-zk

test-local:
	ZK_HOSTS=192.168.99.100:2181 \
	${GODEP} go test ./... -check.vv -v -logtostderr ${TEST_ARGS}


test-zk:
	${GODEP} go test ./... -check.vv -v -logtostderr ${TEST_ARGS}
