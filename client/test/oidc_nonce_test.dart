import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:jose/jose.dart';
import 'package:meowgram_client/src/auth/oidc_service.dart';
import 'support/oidc_fixture.dart';

void main() {
  Future<void> exchange(
    String? idToken, {
    String nonce = 'expected-nonce',
  }) async {
    final service = OidcService(
      httpClient: MockClient((req) async {
        if (req.url.path.endsWith('/jwks.json')) {
          return http.Response(publicKeys(), 200);
        }
        if (req.method == 'GET') return http.Response('{}', 404);
        return http.Response(
          jsonEncode({
            'access_token': 'scratch-access',
            'expires_in': 3600,
            if (idToken != null) 'id_token': idToken,
          }),
          200,
        );
      }),
    );
    await service.exchangeCodeForToken(
      code: 'code',
      codeVerifier: 'verifier',
      redirectUri: 'http://localhost/callback',
      expectedNonce: nonce,
    );
  }

  test('accepts a signed ID token bound to this login', () async {
    await exchange(signedIdToken());
  });
  for (final entry in <String, Map<String, dynamic>>{
    'wrong nonce': {'nonce': 'other-login'},
    'missing nonce': {'nonce': null},
    'wrong issuer': {'iss': 'https://untrusted.example.home.arpa'},
    'wrong audience': {'aud': 'another-client'},
    'multiple audiences without azp': {
      'aud': ['meowgram', 'another-client'],
    },
    'wrong authorized party': {'azp': 'another-client'},
    'expired': {'exp': 1},
    'future issuance': {'iat': 9999999999},
    'missing subject': {'sub': ''},
  }.entries) {
    test('rejects ${entry.key}', () async {
      await expectLater(
        exchange(signedIdToken(claims: entry.value)),
        throwsFormatException,
      );
    });
  }
  test(
    'rejects missing token, malformed token, empty nonce and forged signature',
    () async {
      await expectLater(exchange(null), throwsFormatException);
      await expectLater(exchange('not-a-token'), throwsFormatException);
      await expectLater(
        exchange(signedIdToken(), nonce: ''),
        throwsFormatException,
      );
      final forgedKey = JsonWebKey.generate('RS256', keyBitLength: 2048);
      await expectLater(
        exchange(signedIdToken(key: forgedKey)),
        throwsFormatException,
      );
    },
  );
}
