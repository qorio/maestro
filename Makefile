.SILENT:
.PHONY: _pwd_prompt dec enc

all : $(SUBDIRS)
$(SUBDIRS) :
	$(MAKE) -C $@ all

# 'private' task for echoing instructions
_pwd_prompt: mk_dirs

# Make directories based the file paths
mk_dirs:
	@mkdir -p encrypt decrypt ;

# Decrypt files in the encrypt/ directory
decrypt: _pwd_prompt
	@echo "Decrypt the files in a given directory (those with .cast5 extension)."
	@read -p "Source directory: " src && read -p "Password: " password ; \
	mkdir -p decrypt/$${src} && echo "\n" ; \
	for i in `ls encrypt/$${src}/*.cast5` ; do \
		echo "Decrypting $${i}" ; \
		openssl cast5-cbc -d -in $${i} -out decrypt/$${src}/`basename $${i%.*}` -pass pass:$${password}; \
		chmod 600 decrypt/$${src}/`basename $${i%.*}` ; \
	done ; \
	echo "Decrypted files are in decrypt/$${src}"

# Encrypt files in the decrypt/ directory
encrypt: _pwd_prompt
	@echo "Encrypt the files in a directory using a password you specify.  A directory will be created under /encrypt."
	@read -p "Source directory name: " src && read -p "Password: " password && echo "\n"; \
	mkdir -p encrypt/$${src} ; \
	echo "Encrypting $${src} ==> encrypt/$${src}" ; \
	for i in `ls $${src}` ; do \
		echo "Encrypting $${src}/$${i}" ; \
		openssl cast5-cbc -e -in $${src}/$${i} -out encrypt/$${src}/$${i}.cast5 -pass pass:$${password}; \
	done ; \
	echo "Encrypted files are in encrypt/$${src}"

test-all:
	${GODEP} go test ./pkg/... -v -check.vv


TAG:=`git describe --abbrev=0 --tags`
NOW:=`date -u +%Y-%m-%d_%H-%M-%S`
LDFLAGS:=-X main.BUILD_VERSION $(TAG) -X main.BUILD_TIMESTAMP $(NOW)
TARGET:=main/zkstart.go

pubsubsh:
	echo "Building pubsubsh"
	godep go build -o bin/pubsubsh -ldflags "$(LDFLAGS)" main/pubsubsh.go

zkstart:
	echo "Building zkstart"
	godep go build -o bin/zkstart -ldflags "$(LDFLAGS)" main/zkstart.go

dist-clean:
	rm -rf dist
	rm -f zkstart-linux-*.tar.gz
	rm -f zkstart-darwin-*.tar.gz

dist-linux: dist-clean
	mkdir -p dist/linux/amd64 && GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" "$(TARGET)" -o dist/linux/amd64/zkstart
	mkdir -p dist/linux/i386  && GOOS=linux GOARCH=386 go build -ldflags "$(LDFLAGS)" "$(TARGET)" -o dist/linux/i386/zkstart

dist-osx: dist-clean
	mkdir -p dist/darwin/amd64 && GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" "$(TARGET)" -o dist/darwin/amd64/zkstart
	#mkdir -p dist/darwin/i386  && GOOS=darwin GOARCH=386 go build -ldflags "$(LDFLAGS)" "$(TARGET)" -o dist/darwin/i386/zkstart

release: dist-linux dist-osx
	tar -cvzf zkstart-linux-amd64-$(TAG).tar.gz -C dist/linux/amd64 zkstart
	tar -cvzf zkstart-linux-i386-$(TAG).tar.gz -C dist/linux/i386 zkstart
	tar -cvzf zkstart-darwin-amd64-$(TAG).tar.gz -C dist/darwin/amd64 zkstart
	tar -cvzf zkstart-darwin-i386-$(TAG).tar.gz -C dist/darwin/i386 zkstart
