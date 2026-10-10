import 'dart:convert';
import 'package:jose/jose.dart';
import 'package:meowgram_client/src/config/app_config.dart';

// Disposable keys are generated in memory; no provider credentials are needed.
final signingKey = JsonWebKey.generate('RS256', keyBitLength: 2048);

String publicKeys() {
  final key = signingKey.toJson();
  return jsonEncode({
    'keys': [
      {'kty': 'RSA', 'n': key['n'], 'e': key['e']},
    ],
  });
}

String signedIdToken({
  String nonce = 'expected-nonce',
  Map<String, dynamic> claims = const {},
  JsonWebKey? key,
}) {
  final now = DateTime.now().millisecondsSinceEpoch ~/ 1000;
  final builder = JsonWebSignatureBuilder()
    ..jsonContent = {
      'iss': AppConfig.autheliaIssuerUrl,
      'aud': AppConfig.autheliaClientId,
      'sub': 'scratch-user',
      'iat': now,
      'exp': now + 3600,
      'nonce': nonce,
      ...claims,
    }
    ..addRecipient(key ?? signingKey, algorithm: 'RS256');
  return builder.build().toCompactSerialization();
}
