.PHONY: _pwd_prompt dec enc

SECRET_FILES= docker/qoriolabs.dockercfg environments/dev/.ssh/gce-qoriolabsdev


# 'private' task for echoing instructions
_pwd_prompt: echo
	@echo "Contact me for password."

dec: _pwd_prompt
	for i in ${SECRET_FILES} ; do \
		echo "Decrypting $${i}" ; \
		openssl cast5-cbc -d -in encrypt/$${i}.cast5 -out decrypt/$${i} ; \
		chmod 600 $${i} ; \
	done

enc: _pwd_prompt
	for i in ${SECRET_FILES} ; do \
		echo "Encrypting $${i}" ; \
		openssl cast5-cbc -e -in decrypt/$${i} -out encrypt/$${i}.cast5 ; \
	done


echo: _pwd_prompt
	for i in ${SECRET_FILES} ; do \
		echo $$i ; \
		mkdir -p encrypt/$$(dirname $$i); \
		mkdir -p decrypt/$$(dirname $$i); \
	done
