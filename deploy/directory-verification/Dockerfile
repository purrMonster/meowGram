FROM lldap/lldap:v0.6.3@sha256:2a8454b668c1aba7157e832eab0e242e1e7eb5fb7591d7e7774ba05286511ca8
USER root
RUN apk add --no-cache python3 py3-ldap3 py3-cryptography py3-requests
COPY verify.py /verify.py
USER 1000:1000
ENTRYPOINT ["python3", "/verify.py"]
