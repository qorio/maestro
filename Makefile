.PHONY: _pwd_prompt dec enc

# Files that are encrypted
SECRET_FILES= docker/qoriolabs.dockercfg environments/dev/.ssh/gce-qoriolabsdev


# 'private' task for echoing instructions
_pwd_prompt: mk_dirs
	@echo "Ask me"

# Make directories based the file paths
mk_dirs: _pwd_prompt
	for i in ${SECRET_FILES} ; do \
		echo $$i ; \
		mkdir -p encrypt/$$(dirname $$i) ; \
		mkdir -p decrypt/$$(dirname $$i) ; \
	done

# Decrypt files in the encrypt/ directory
dec: _pwd_prompt
	@read -s -p "Enter the password: " password ; \
	for i in ${SECRET_FILES} ; do \
		echo "Decrypting encrypt/$${i}" ; \
		openssl cast5-cbc -d -in encrypt/$${i}.cast5 -out decrypt/$${i} -pass pass:$${password}; \
		chmod 600 decrypt/$${i} ; \
	done

# Encrypt files in the decrypt/ directory
enc: _pwd_prompt
	@read -s -p "Enter the password: " password ; \
	for i in ${SECRET_FILES} ; do \
		echo "Encrypting decrypt/$${i}" ; \
		openssl cast5-cbc -e -in decrypt/$${i} -out encrypt/$${i}.cast5 -pass pass:$${password}; \
	done

