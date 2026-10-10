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

  /// Factory deserializer from OAuth 2.0 token response or cached secure storage.
  factory TokenData.fromJson(Map<String, dynamic> json) {
    DateTime parsedExpiresAt;
    if (json['expires_at'] != null) {
      parsedExpiresAt =
          DateTime.tryParse(json['expires_at'].toString()) ??
          DateTime.now().add(const Duration(hours: 1));
    } else {
      final expiresInSec = (json['expires_in'] as num?)?.toInt() ?? 3600;
      parsedExpiresAt = DateTime.now().add(Duration(seconds: expiresInSec));
    }

    return TokenData(
      accessToken: json['access_token'] as String? ?? '',
      idToken: json['id_token'] as String?,
      refreshToken: json['refresh_token'] as String?,
      tokenType: json['token_type'] as String? ?? 'Bearer',
      expiresAt: parsedExpiresAt,
    );
  }

  /// Serializes token metadata to JSON for secure persistent storage.
  Map<String, dynamic> toJson() => {
    'access_token': accessToken,
    if (idToken != null) 'id_token': idToken,
    if (refreshToken != null) 'refresh_token': refreshToken,
    'token_type': tokenType,
    'expires_at': expiresAt.toUtc().toIso8601String(),
  };

  /// Whether the access token is close to expiry or already expired (within 60 second buffer).
  bool get isExpired =>
      DateTime.now().isAfter(expiresAt.subtract(const Duration(seconds: 60)));

  /// Remaining duration until token expiration.
  Duration get timeUntilExpiry => expiresAt.difference(DateTime.now());
}

/// User identity claims parsed from Authelia's OIDC access token or ID token.
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

  static Map<String, dynamic>? _decodeJwtPayload(String? jwtString) {
    if (jwtString == null || jwtString.isEmpty) return null;
    try {
      final parts = jwtString.split('.');
      if (parts.length != 3) return null;

      // Base64URL decode middle payload segment with normalized padding
      var normalized = parts[1].replaceAll('-', '+').replaceAll('_', '/');
      while (normalized.length % 4 != 0) {
        normalized += '=';
      }

      final payloadBytes = base64.decode(normalized);
      return jsonDecode(utf8.decode(payloadBytes)) as Map<String, dynamic>;
    } catch (_) {
      return null;
    }
  }

  /// Extracts user claims, prioritizing the Authelia access token claims first,
  /// then ID token, resolving: preferred_username -> name -> email prefix -> sub UUID.
  factory UserProfile.fromTokens({
    required String? accessToken,
    String? idToken,
  }) {
    final accessClaims = _decodeJwtPayload(accessToken) ?? {};
    final idClaims = _decodeJwtPayload(idToken) ?? {};

    // 1. Subject extraction (fallback: access -> id -> anonymous)
    final sub =
        accessClaims['sub']?.toString() ??
        idClaims['sub']?.toString() ??
        'anonymous';

    // 2. Candidate username fields prioritized from access token, then id token
    final preferred =
        accessClaims['preferred_username']?.toString() ??
        idClaims['preferred_username']?.toString();

    final name =
        accessClaims['name']?.toString() ?? idClaims['name']?.toString();

    final email =
        accessClaims['email']?.toString() ?? idClaims['email']?.toString();

    // 3. Username hierarchy resolution:
    // preferred_username -> name -> email prefix -> sub UUID
    String resolvedUsername;
    if (preferred != null && preferred.trim().isNotEmpty) {
      resolvedUsername = preferred.trim();
    } else if (name != null && name.trim().isNotEmpty) {
      resolvedUsername = name.trim();
    } else if (email != null && email.contains('@')) {
      final prefix = email.split('@')[0].trim();
      resolvedUsername = prefix.isNotEmpty ? prefix : sub;
    } else {
      resolvedUsername = sub;
    }

    return UserProfile(
      sub: sub,
      username: resolvedUsername,
      email: email,
      name: name,
    );
  }

  /// Backwards-compatible factory delegating to [fromTokens].
  factory UserProfile.fromJwt(String? jwtString) {
    return UserProfile.fromTokens(accessToken: jwtString);
  }
}
