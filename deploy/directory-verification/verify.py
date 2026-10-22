"""LLDAP compatibility probe. Run only in the disposable --network none image.

All credentials and certificates are generated in container tmpfs. Provider logs,
HTTP bodies, and exceptions are deliberately excluded from output.
"""
import datetime
import ipaddress
import os
from pathlib import Path
import secrets
import ssl
import subprocess
import sys
import tempfile
import time
import uuid

import ldap3
from ldap3.utils.conv import escape_filter_chars
import requests
from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import NameOID

BASE = "dc=example,dc=com"
HTTP = "http://127.0.0.1:17170"
stage = "initialization"


class BindRejected(Exception):
    pass


def check(condition, label):
    if not condition:
        raise RuntimeError(label)
    print("PASS " + label, flush=True)


def main():
    global stage
    os.umask(0o077)
    with tempfile.TemporaryDirectory() as directory:
        root = Path(directory)
        admin_password = secrets.token_urlsafe(32)
        bot_password = secrets.token_urlsafe(32)
        reader_password = secrets.token_urlsafe(32)
        key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
        name = x509.Name([x509.NameAttribute(NameOID.COMMON_NAME, "localhost")])
        now = datetime.datetime.now(datetime.timezone.utc)
        cert = (x509.CertificateBuilder().subject_name(name).issuer_name(name)
                .public_key(key.public_key()).serial_number(x509.random_serial_number())
                .not_valid_before(now - datetime.timedelta(minutes=1))
                .not_valid_after(now + datetime.timedelta(days=1))
                .add_extension(x509.BasicConstraints(ca=True, path_length=None), critical=True)
                .add_extension(x509.SubjectAlternativeName([
                    x509.DNSName("localhost"), x509.IPAddress(ipaddress.ip_address("127.0.0.1"))]), critical=False)
                .sign(key, hashes.SHA256()))
        (root / "cert.pem").write_bytes(cert.public_bytes(serialization.Encoding.PEM))
        (root / "key.pem").write_bytes(key.private_bytes(
            serialization.Encoding.PEM, serialization.PrivateFormat.PKCS8,
            serialization.NoEncryption()))
        config = root / "lldap.toml"
        config.write_text('\n'.join([
            'ldap_host = "127.0.0.1"', 'http_host = "127.0.0.1"',
            'ldap_port = 3890', 'http_port = 17170',
            'http_url = "http://localhost:17170"',
            f'ldap_base_dn = "{BASE}"', 'ldap_user_dn = "admin"',
            f'ldap_user_pass = "{admin_password}"',
            f'jwt_secret = "{secrets.token_urlsafe(32)}"',
            f'key_seed = "{secrets.token_urlsafe(32)}"',
            f'database_url = "sqlite://{root}/users.db?mode=rwc"',
            '[ldaps_options]', 'enabled = true', 'port = 6360',
            f'cert_file = "{root}/cert.pem"', f'key_file = "{root}/key.pem"',
        ]))
        stage = "provider startup"
        process = subprocess.Popen(["/app/lldap", "run", "--config-file", str(config)],
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            token = None
            for _ in range(60):
                if process.poll() is not None:
                    raise RuntimeError("provider exited")
                try:
                    response = requests.post(HTTP + "/auth/simple/login", json={
                        "username": "admin", "password": admin_password}, timeout=2)
                    if response.ok:
                        token = response.json()["token"]
                        break
                except requests.RequestException:
                    pass
                time.sleep(0.5)
            check(token is not None, "pinned LLDAP starts in isolated container")

            def graphql(query, variables=None):
                response = requests.post(HTTP + "/api/graphql", json={
                    "query": query, "variables": variables or {}},
                    headers={"Authorization": "Bearer " + token}, timeout=5)
                response.raise_for_status()
                body = response.json()
                if body.get("errors"):
                    raise RuntimeError("GraphQL operation failed")
                return body["data"]

            def set_password(user, password):
                env = dict(os.environ, LLDAP_USER_PASSWORD=password)
                subprocess.run(["/app/lldap_set_password", "--base-url", HTTP,
                                "--token", token, "--username", user], env=env,
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                               check=True, timeout=15)

            def create(user):
                return graphql('mutation($u: CreateUserInput!) {createUser(user:$u){uuid}}',
                               {"u": {"id": user, "email": user + "@example.com",
                                      "displayName": user}})["createUser"]["uuid"]

            def membership(user, group, add=True):
                operation = "addUserToGroup" if add else "removeUserFromGroup"
                graphql(f'mutation($u:String!,$g:Int!) {{{operation}(userId:$u,groupId:$g){{ok}}}}',
                        {"u": user, "g": group})

            def connect(user, password, trusted=True):
                tls = ldap3.Tls(validate=ssl.CERT_REQUIRED,
                                ca_certs_file=str(root / "cert.pem") if trusted else None)
                server = ldap3.Server("localhost", port=6360, use_ssl=True,
                                      tls=tls, connect_timeout=3, get_info=ldap3.NONE)
                connection = ldap3.Connection(server, user=f"uid={user},ou=people,{BASE}",
                                              password=password, receive_timeout=3)
                if not connection.bind():
                    result_code = connection.result.get("result")
                    connection.unbind()
                    if result_code != 49:
                        raise RuntimeError("unexpected bind failure")
                    raise BindRejected()
                return connection

            def lookup():
                connection = connect("reader", reader_password)
                try:
                    ok = connection.search(f"ou=people,{BASE}",
                        "(uid=" + escape_filter_chars("probe-bot") + ")",
                        attributes=["entryUUID", "uid", "cn", "memberOf"])
                    if not ok or len(connection.entries) != 1:
                        raise RuntimeError("account resolution failed")
                    return connection.entries[0].entry_attributes_as_dict
                finally:
                    connection.unbind()

            stage = "fixture provisioning"
            original_uuid = create("probe-bot")
            uuid.UUID(original_uuid)
            create("reader")
            groups = graphql('{groups{id displayName}}')["groups"]
            readonly = next(g["id"] for g in groups if g["displayName"] == "lldap_strict_readonly")
            membership("reader", readonly)
            group = graphql('mutation {createGroup(name:"meowgram-api-bots"){id}}')["createGroup"]["id"]
            membership("probe-bot", group)
            set_password("probe-bot", bot_password)
            set_password("reader", reader_password)

            stage = "verified LDAPS and reader lookup"
            connection = connect("probe-bot", bot_password)
            connection.unbind()
            check(True, "user bind over certificate-verified LDAPS")
            entry = lookup()
            check(str(entry["entryUUID"][0]) == original_uuid,
                  "strict-readonly reader resolves LDAP entryUUID matching provider UUID")
            group_dn = f"cn=meowgram-api-bots,ou=groups,{BASE}"
            check(group_dn in entry["memberOf"], "direct memberOf contains allowed group DN")
            stage = "invalid authentication and trust"
            for label, password, trusted, expected in [
                ("wrong password rejected", secrets.token_urlsafe(32), True, BindRejected),
                ("untrusted TLS certificate rejected", bot_password, False,
                 ldap3.core.exceptions.LDAPSocketOpenError),
            ]:
                rejected = False
                try:
                    connection = connect("probe-bot", password, trusted)
                    connection.unbind()
                except expected:
                    rejected = True
                check(rejected, label)

            stage = "StartTLS capability"
            connection = ldap3.Connection(ldap3.Server("localhost", port=3890,
                tls=ldap3.Tls(validate=ssl.CERT_REQUIRED, ca_certs_file=str(root / "cert.pem")),
                connect_timeout=3, get_info=ldap3.NONE), receive_timeout=3)
            try:
                connection.open()
                try:
                    supported = connection.start_tls()
                except ldap3.core.exceptions.LDAPStartTLSError:
                    supported = False
                check(connection.result.get("result") in (2, 12, 53),
                      "StartTLS rejected by server protocol response")
                check(not supported, "v0.6.3 StartTLS unavailable; no bind or plaintext fallback attempted")
            finally:
                connection.unbind()

            stage = "identity lifecycle"
            graphql('mutation($u:UpdateUserInput!){updateUser(user:$u){ok}}',
                    {"u": {"id": "probe-bot", "displayName": "Renamed bot"}})
            entry = lookup()
            check(str(entry["entryUUID"][0]) == original_uuid, "display-name change preserves entryUUID")
            fields = graphql('{__type(name:"UpdateUserInput"){inputFields{name}}}')["__type"]["inputFields"]
            check({f["name"] for f in fields}.isdisjoint({"newId", "newUserId", "uuid"}),
                  "provider update contract has no login-ID rename or UUID setter")
            membership("probe-bot", group, False)
            check(group_dn not in lookup().get("memberOf", []), "group removal visible on next reader bind/search")
            rotated = secrets.token_urlsafe(32)
            set_password("probe-bot", rotated)
            rejected = False
            try:
                connection = connect("probe-bot", bot_password)
                connection.unbind()
            except BindRejected:
                rejected = True
            check(rejected, "password rotation rejects old password")
            connection = connect("probe-bot", rotated)
            connection.unbind()
            check(True, "rotated password accepted")
            graphql('mutation {deleteUser(userId:"probe-bot"){ok}}')
            recreated_uuid = create("probe-bot")
            check(recreated_uuid != original_uuid, "deleted/recreated login gets a different entryUUID")
            check(str(lookup()["entryUUID"][0]) == recreated_uuid, "LDAP exposes recreated identity")
            check(group_dn not in lookup().get("memberOf", []), "recreated account does not inherit bot group")
            print("PASS directory compatibility probe complete", flush=True)
        finally:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print("FAIL " + stage + " (details suppressed to protect generated credentials)", file=sys.stderr)
        sys.exit(1)
