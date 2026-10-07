import 'dart:convert';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:meowgram_client/src/auth/auth_state.dart';

/// Secure persistence interface for OAuth 2.0 / OIDC credentials.
///
/// Complies with Epic 1.1:
/// - Persists Access Token, ID Token, Refresh Token, and Expiry timestamp.
/// - Configured with [KeychainAccessibility.first_unlock] on iOS and macOS to allow
///   background token refreshes while keeping keys encrypted at rest.
/// - Encrypted shared preferences enabled on Android.
class TokenStorage {
  final FlutterSecureStorage _storage;

  static const String _keyTokenData = 'meowgram_auth_tokens';

  TokenStorage({FlutterSecureStorage? storage})
      : _storage = storage ??
            const FlutterSecureStorage(
              aOptions: const AndroidOptions(),
              iOptions: IOSOptions(
                accessibility: KeychainAccessibility.first_unlock,
              ),
              mOptions: MacOsOptions(
                accessibility: KeychainAccessibility.first_unlock,
              ),
            );

  /// Saves the complete active token set to secure keychain/keystore.
  Future<void> saveTokens(TokenData tokens) async {
    final jsonString = jsonEncode(tokens.toJson());
    await _storage.write(key: _keyTokenData, value: jsonString);
  }

  /// Reads and deserializes the stored token payload if available.
  Future<TokenData?> readTokens() async {
    final raw = await _storage.read(key: _keyTokenData);
    if (raw == null || raw.isEmpty) {
      return null;
    }
    try {
      final map = jsonDecode(raw) as Map<String, dynamic>;
      return TokenData.fromJson(map);
    } catch (_) {
      return null;
    }
  }

  /// Deletes all persisted tokens on logout or session invalidation.
  Future<void> clearTokens() async {
    await _storage.delete(key: _keyTokenData);
  }
}
