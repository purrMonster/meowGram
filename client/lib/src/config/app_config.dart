import 'package:flutter/foundation.dart';

/// Runtime and compile-time configuration contract for the meowGram client.
///
/// In alignment with the "Zero Hardcoded Domains" architectural mandate,
/// all service targets, authentication parameters, and domain references
/// are sourced strictly via compile-time `--dart-define` declarations.
class AppConfig {
  const AppConfig._();

  /// Canonical application brand name displayed in the UI.
  static const String appName = 'meowGram';

  /// Environment mode: 'development', 'staging', or 'production'.
  static const String environment = String.fromEnvironment(
    'APP_ENV',
    defaultValue: 'development',
  );

  /// Target application domain (e.g., 'localhost', 'meowgram.local', 'chat.purrbrews.com').
  static const String appDomain = String.fromEnvironment(
    'APP_DOMAIN',
    defaultValue: 'localhost',
  );

  /// Target HTTP port for the backend service (default: 8080).
  static const String port = String.fromEnvironment(
    'HTTP_PORT',
    defaultValue: '8080',
  );

  /// Whether to enforce secure protocols (https:// and wss://) instead of http/ws.
  static const bool useSecureSchemes = bool.fromEnvironment(
    'USE_SECURE_SCHEMES',
    defaultValue: false,
  );

  /// Optional direct override for HTTP API base URL.
  static const String _apiBaseUrlOverride = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: '',
  );

  /// Optional direct override for WebSocket endpoint URL.
  static const String _wsBaseUrlOverride = String.fromEnvironment(
    'WS_BASE_URL',
    defaultValue: '',
  );

  // ===========================================================================
  // Authelia OpenID Connect (OIDC) PKCE Configuration
  // ===========================================================================

  /// Client ID registered in Authelia's identity provider configuration.
  ///
  /// In an OIDC Authorization Code Flow with PKCE, this is a public client identifier.
  /// No client secret is ever stored or transmitted by the client.
  static const String autheliaClientId = String.fromEnvironment(
    'AUTHELIA_CLIENT_ID',
    defaultValue: 'meowgram-client',
  );

  /// Base issuer URL for Authelia's OpenID Connect provider.
  ///
  /// Used to discover the OpenID configuration (`/.well-known/openid-configuration`)
  /// and target the authorization and token exchange endpoints.
  static const String autheliaIssuerUrl = String.fromEnvironment(
    'AUTHELIA_ISSUER_URL',
    defaultValue: 'http://localhost:9091',
  );

  /// Optional explicit redirect URI override.
  ///
  /// If omitted, [authRedirectUri] dynamically determines the appropriate URI:
  /// - Web: Resolves to `Uri.base.origin` (e.g. `http://localhost:8080` or hosting domain).
  /// - Desktop: Resolves to loopback callback `http://127.0.0.1:8088/callback` (per RFC 8252).
  static const String _authRedirectUriOverride = String.fromEnvironment(
    'AUTH_REDIRECT_URI',
    defaultValue: '',
  );

  /// Resolved OAuth 2.0 / OIDC redirect URI based on platform context.
  static String get authRedirectUri {
    if (_authRedirectUriOverride.isNotEmpty) {
      return _authRedirectUriOverride;
    }
    if (kIsWeb) {
      // In Flutter Web, the callback lands back on the host origin
      final uri = Uri.base;
      return '${uri.scheme}://${uri.host}${uri.hasPort ? ':${uri.port}' : ''}';
    }
    // For Desktop platforms (Windows, macOS, Linux), standard loopback is used per RFC 8252
    return 'http://127.0.0.1:8088/callback';
  }

  /// OIDC scopes requested during authorization.
  /// - `openid`: Required for OIDC identity.
  /// - `profile`: Requests user details like preferred_username and name.
  /// - `email`: Requests user's verified email.
  /// - `offline_access`: Requests a refresh token for session continuity.
  static const List<String> oidcScopes = <String>[
    'openid',
    'profile',
    'email',
    'offline_access',
  ];

  // ===========================================================================
  // Backend Service Endpoint Resolvers
  // ===========================================================================

  /// Resolves the effective HTTP Base URL.
  static String get apiBaseUrl {
    if (_apiBaseUrlOverride.isNotEmpty) {
      return _apiBaseUrlOverride;
    }
    final scheme = useSecureSchemes ? 'https' : 'http';
    final hasStandardPort = port.isEmpty || port == '80' || port == '443';
    return hasStandardPort
        ? '$scheme://$appDomain'
        : '$scheme://$appDomain:$port';
  }

  /// Resolves the effective WebSocket endpoint URL.
  static String get wsBaseUrl {
    if (_wsBaseUrlOverride.isNotEmpty) {
      return _wsBaseUrlOverride;
    }
    final scheme = useSecureSchemes ? 'wss' : 'ws';
    final hasStandardPort = port.isEmpty || port == '80' || port == '443';
    final host = hasStandardPort ? appDomain : '$appDomain:$port';
    return '$scheme://$host/ws';
  }

  /// Builds a WebSocket URL with an authenticated Bearer token query parameter.
  ///
  /// Since web browsers cannot inject custom `Authorization` headers into the
  /// WebSocket handshake request, the backend contract supports authentication via
  /// `?token={accessToken}`.
  static String authenticatedWsUrl(String accessToken) {
    final base = wsBaseUrl;
    final separator = base.contains('?') ? '&' : '?';
    return '$base${separator}token=${Uri.encodeComponent(accessToken)}';
  }

  /// Service health-check endpoint URL.
  static String get healthCheckUrl => '$apiBaseUrl/healthz';

  /// Convenience helper indicating if running under development environment.
  static bool get isDevelopment => environment == 'development';
}
