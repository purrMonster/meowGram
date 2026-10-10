import 'dart:convert';
import 'dart:math';
import 'package:crypto/crypto.dart';

/// Cryptographic helper for OAuth 2.0 Proof Key for Code Exchange (PKCE)
/// per RFC 7636 specification.
///
/// Mechanics:
/// 1. A high-entropy cryptographic [codeVerifier] is generated on the client.
/// 2. A SHA-256 digest is taken of the ASCII-encoded verifier, then base64url-encoded
///    without padding to produce the [codeChallenge].
/// 3. The authorization request sends [codeChallenge] and `code_challenge_method=S256`.
/// 4. The token exchange sends the original raw [codeVerifier].
/// 5. The server independently calculates SHA-256(code_verifier) and compares it with
///    the challenge stored from step 3.
/// This prevents authorization code interception attacks on public clients where
/// client secrets cannot be securely kept.
class PkcePair {
  final String codeVerifier;
  final String codeChallenge;
  final String codeChallengeMethod;
  final String state;
  final String nonce;

  const PkcePair({
    required this.codeVerifier,
    required this.codeChallenge,
    this.codeChallengeMethod = 'S256',
    required this.state,
    required this.nonce,
  });

  /// Generates a fresh RFC 7636 compliant PKCE pair with a random state and nonce.
  factory PkcePair.generate({int length = 64}) {
    final verifier = _generateRandomString(length);
    final challenge = _computeS256Challenge(verifier);
    final state = _generateRandomString(32);
    final nonce = _generateRandomString(32);

    return PkcePair(
      codeVerifier: verifier,
      codeChallenge: challenge,
      codeChallengeMethod: 'S256',
      state: state,
      nonce: nonce,
    );
  }

  /// Computes BASE64URL-ENCODE(SHA256(ASCII(code_verifier))) without padding per RFC 7636 Section 4.2.
  static String _computeS256Challenge(String verifier) {
    final bytes = ascii.encode(verifier);
    final digest = sha256.convert(bytes);
    // Base64URL encode and strip trailing '=' padding per RFC 7636
    return base64Url.encode(digest.bytes).replaceAll('=', '');
  }

  /// Generates a high-entropy cryptographically secure random string using unreserved characters.
  static String _generateRandomString(int length) {
    const charset =
        'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~';
    final random = Random.secure();
    return List.generate(
      length,
      (_) => charset[random.nextInt(charset.length)],
    ).join();
  }
}
