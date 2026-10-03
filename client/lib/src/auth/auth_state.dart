import 'dart:convert';

/// High-level authentication state of the meowGram client.
enum AuthStatus {
  /// User is not logged in; access token is absent or has expired.
  unauthenticated,

  /// Authentication flow is actively running (browser open, waiting for PKCE callback).
  authenticating,

  /// User holds a verified, active access token ready for API and WebSocket streaming.
  authenticated,

  /// Authentication or token refresh encountered an error.
  error,
}

/// Token payload received from Authelia's OIDC token endpoint.
class TokenData {
  final String accessToken;
  final String? idToken;
  final String? refreshToken;
  final String tokenType;
  final DateTime expiresAt;

  const TokenData({
    required this.accessToken,
    this.idToken,
    this.refreshToken,
    this.tokenType = 'Bearer',
    required this.expiresAt,
  });

  /// Factory deserializer from standard OAuth 2.0 token response JSON.
  factory TokenData.fromJson(Map<String, dynamic> json) {
    final expiresInSec = (json['expires_in'] as num?)?.toInt() ?? 3600;
    return TokenData(
      accessToken: json['access_token'] as String? ?? '',
      idToken: json['id_token'] as String?,
      refreshToken: json['refresh_token'] as String?,
      tokenType: json['token_type'] as String? ?? 'Bearer',
      expiresAt: DateTime.now().add(Duration(seconds: expiresInSec)),
    );
  }

  /// Whether the access token is close to expiry or already expired (within 60 second buffer).
  bool get isExpired =>
      DateTime.now().isAfter(expiresAt.subtract(const Duration(seconds: 60)));

  /// Remaining duration until token expiration.
  Duration get timeUntilExpiry => expiresAt.difference(DateTime.now());
}

/// User identity claims parsed from Authelia's OIDC ID token or access token.
class UserProfile {
  final String sub;
  final String username;
  final String? email;
  final String? name;

  const UserProfile({
    required this.sub,
    required this.username,
    this.email,
    this.name,
  });

  /// Extracts claims by decoding JWT payload segments without external signature verification
  /// (backend verifies signature cryptographically; client only reads display metadata).
  factory UserProfile.fromJwt(String? jwtString) {
    if (jwtString == null || jwtString.isEmpty) {
      return const UserProfile(sub: 'anonymous', username: 'Anonymous Cat');
    }

    try {
      final parts = jwtString.split('.');
      if (parts.length != 3) {
        return const UserProfile(sub: 'unknown', username: 'purrUser');
      }

      // Base64URL decode middle payload segment with normalized padding
      var normalized = parts[1].replaceAll('-', '+').replaceAll('_', '/');
      while (normalized.length % 4 != 0) {
        normalized += '=';
      }

      final payloadBytes = base64.decode(normalized);
      final payloadMap =
          jsonDecode(utf8.decode(payloadBytes)) as Map<String, dynamic>;

      final sub = payloadMap['sub']?.toString() ?? 'unknown';
      final preferred = payloadMap['preferred_username']?.toString();
      final name = payloadMap['name']?.toString();
      final email = payloadMap['email']?.toString();

      // Resolve username hierarchy: preferred_username -> name -> email prefix -> sub
      final resolvedUsername = (preferred != null && preferred.isNotEmpty)
          ? preferred
          : (name != null && name.isNotEmpty)
              ? name
              : (email != null && email.contains('@'))
                  ? email.split('@')[0]
                  : sub;

      return UserProfile(
        sub: sub,
        username: resolvedUsername,
        email: email,
        name: name,
      );
    } catch (_) {
      return const UserProfile(sub: 'unknown', username: 'purrUser');
    }
  }
}
