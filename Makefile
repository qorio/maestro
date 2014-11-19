.PHONY: _pwd_prompt dec enc

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
