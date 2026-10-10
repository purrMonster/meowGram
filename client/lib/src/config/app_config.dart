import 'package:flutter/foundation.dart';

/// Runtime and compile-time configuration contract for the meowGram client.
///
/// In alignment with the "Zero Hardcoded Domains" architectural mandate,
/// all service targets, authentication parameters, endpoints, and domain references
/// are read strictly via compile-time `const` environment readers (`String.fromEnvironment`,
/// `bool.fromEnvironment`). This ensures full compatibility with `--dart-define`
/// and `--dart-define-from-file` without runtime evaluation drops.
class AppConfig {
  const AppConfig._();

  // ===========================================================================
  // Strict Compile-Time `const` Environment Readers
  // ===========================================================================

  /// Canonical application brand name displayed in the UI.
  static const String appName = 'meowGram';

  /// Environment mode: 'development', 'staging', or 'production'.
  /// Sourced strictly at compile-time from `--dart-define=APP_ENV=...`
  static const String environment = String.fromEnvironment(
    'APP_ENV',
    defaultValue: 'development',
  );

  /// Target application domain (e.g., 'localhost', 'meow.example.home.arpa').
  /// Sourced strictly at compile-time from `--dart-define=APP_DOMAIN=...`
  static const String appDomain = String.fromEnvironment(
    'APP_DOMAIN',
    defaultValue: 'localhost',
  );

  /// Target HTTP port for backend services (default: 8080).
  /// Sourced strictly at compile-time from `--dart-define=HTTP_PORT=...`
  static const int httpPort = int.fromEnvironment(
    'HTTP_PORT',
    defaultValue: 8080,
  );

  /// Target HTTP port as string representation.
  static String get port => httpPort.toString();

  /// Whether to enforce secure protocols (https:// and wss://) instead of http/ws.
  /// Sourced strictly at compile-time from `--dart-define=USE_SECURE_SCHEMES=...`
  static const bool useSecureSchemes = bool.fromEnvironment(
    'USE_SECURE_SCHEMES',
    defaultValue: false,
  );

  // ===========================================================================
  // Authelia OpenID Connect (OIDC) PKCE Configuration
  // ===========================================================================

  /// Client ID registered in Authelia's identity provider configuration.
  /// Sourced strictly at compile-time from `--dart-define=AUTHELIA_CLIENT_ID=...`
  static const String autheliaClientId = String.fromEnvironment(
    'AUTHELIA_CLIENT_ID',
    defaultValue: 'meowgram',
  );

  /// Base issuer URL for Authelia's OpenID Connect provider.
  /// Sourced strictly at compile-time from `--dart-define=AUTHELIA_ISSUER_URL=...`
  static const String _rawAutheliaIssuerUrl = String.fromEnvironment(
    'AUTHELIA_ISSUER_URL',
    defaultValue: '',
  );

  /// Authelia Domain (e.g., 'auth.example.home.arpa', 'localhost:9091').
  /// Sourced strictly at compile-time from `--dart-define=AUTHELIA_DOMAIN=...`
  static const String autheliaDomain = String.fromEnvironment(
    'AUTHELIA_DOMAIN',
    defaultValue: 'localhost:9091',
  );

  /// Effective Authelia Issuer URL.
  /// Sourced with fallback order:
  /// 1. Compile-time `--dart-define=AUTHELIA_ISSUER_URL`
  /// 2. Inferred from `--dart-define=AUTHELIA_DOMAIN` if set and not default 'localhost:9091'
  /// 3. Default fallback: 'http://localhost:9091'
  static String get autheliaIssuerUrl {
    if (_rawAutheliaIssuerUrl.isNotEmpty) {
      return _rawAutheliaIssuerUrl.replaceAll(RegExp(r'/+$'), '');
    }
    if (autheliaDomain.isNotEmpty && autheliaDomain != 'localhost:9091') {
      if (autheliaDomain.startsWith('http://') || autheliaDomain.startsWith('https://')) {
        return autheliaDomain.replaceAll(RegExp(r'/+$'), '');
      }
      final isLocal = autheliaDomain.contains('localhost') || autheliaDomain.startsWith('127.');
      final scheme = (useSecureSchemes || port == '443' || !isLocal) ? 'https' : 'http';
      return '$scheme://$autheliaDomain';
    }
    return 'http://localhost:9091';
  }

  /// JSON Web Key Set (JWKS) URL used for signature validation.
  static const String _rawAutheliaJwksUrl = String.fromEnvironment(
    'AUTHELIA_JWKS_URL',
    defaultValue: '',
  );
  static String get autheliaJwksUrl => _rawAutheliaJwksUrl.isNotEmpty
      ? _rawAutheliaJwksUrl
      : '$autheliaIssuerUrl/jwks.json';

  /// OpenID Discovery configuration endpoint URL.
  static const String _rawAutheliaDiscoveryUrl = String.fromEnvironment(
    'AUTHELIA_DISCOVERY_URL',
    defaultValue: '',
  );
  static String get autheliaDiscoveryUrl => _rawAutheliaDiscoveryUrl.isNotEmpty
      ? _rawAutheliaDiscoveryUrl
      : '$autheliaIssuerUrl/.well-known/openid-configuration';

  /// Authelia OIDC Authorization endpoint.
  static const String _rawAutheliaAuthEndpoint = String.fromEnvironment(
    'AUTHELIA_AUTHORIZATION_ENDPOINT',
    defaultValue: '',
  );
  static String get autheliaAuthorizationEndpoint => _rawAutheliaAuthEndpoint.isNotEmpty
      ? _rawAutheliaAuthEndpoint
      : '$autheliaIssuerUrl/api/oidc/authorization';

  /// Authelia OIDC Token exchange endpoint.
  static const String _rawAutheliaTokenEndpoint = String.fromEnvironment(
    'AUTHELIA_TOKEN_ENDPOINT',
    defaultValue: '',
  );
  static String get autheliaTokenEndpoint => _rawAutheliaTokenEndpoint.isNotEmpty
      ? _rawAutheliaTokenEndpoint
      : '$autheliaIssuerUrl/api/oidc/token';

  /// Authelia OIDC UserInfo endpoint.
  static const String _rawAutheliaUserinfoEndpoint = String.fromEnvironment(
    'AUTHELIA_USERINFO_ENDPOINT',
    defaultValue: '',
  );
  static String get autheliaUserinfoEndpoint => _rawAutheliaUserinfoEndpoint.isNotEmpty
      ? _rawAutheliaUserinfoEndpoint
      : '$autheliaIssuerUrl/api/oidc/userinfo';

  /// Authelia OIDC Token revocation endpoint.
  static const String _rawAutheliaRevocationEndpoint = String.fromEnvironment(
    'AUTHELIA_REVOCATION_ENDPOINT',
    defaultValue: '',
  );
  static String get autheliaRevocationEndpoint => _rawAutheliaRevocationEndpoint.isNotEmpty
      ? _rawAutheliaRevocationEndpoint
      : '$autheliaIssuerUrl/api/oidc/revocation';

  /// Optional explicit redirect URI override.
  /// Sourced strictly at compile-time from `--dart-define=AUTH_REDIRECT_URI=...`
  static const String _rawAuthRedirectUri = String.fromEnvironment(
    'AUTH_REDIRECT_URI',
    defaultValue: '',
  );

  /// Resolved OAuth 2.0 / OIDC redirect URI based on platform context.
  static String get authRedirectUri {
    if (_rawAuthRedirectUri.isNotEmpty) {
      return _rawAuthRedirectUri;
    }
    if (kIsWeb) {
      final uri = Uri.base;
      return '${uri.scheme}://${uri.host}${uri.hasPort ? ':${uri.port}' : ''}';
    }
    if (!kIsWeb &&
        (defaultTargetPlatform == TargetPlatform.iOS ||
            defaultTargetPlatform == TargetPlatform.android)) {
      return 'meowgram://callback';
    }
    return 'http://127.0.0.1:8088/callback';
  }

  /// OIDC scopes requested during authorization.
  static const List<String> oidcScopes = <String>[
    'openid',
    'profile',
    'email',
    'offline_access',
  ];

  // ===========================================================================
  // Backend Service Endpoint Resolvers
  // ===========================================================================

  /// Optional direct override for HTTP API base URL.
  /// Sourced strictly at compile-time from `--dart-define=API_BASE_URL=...`
  static const String _rawApiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: '',
  );

  /// Resolves the effective HTTP Base URL.
  static String get apiBaseUrl {
    if (_rawApiBaseUrl.isNotEmpty) {
      return _rawApiBaseUrl.replaceAll(RegExp(r'/+$'), '');
    }
    final isSecure = useSecureSchemes || port == '443';
    final scheme = isSecure ? 'https' : 'http';
    final hasStandardPort = port.isEmpty || port == '80' || port == '443';
    return hasStandardPort
        ? '$scheme://$appDomain'
        : '$scheme://$appDomain:$port';
  }

  /// Optional direct override for WebSocket endpoint URL.
  /// Sourced strictly at compile-time from `--dart-define=WS_BASE_URL=...`
  static const String _rawWsBaseUrl = String.fromEnvironment(
    'WS_BASE_URL',
    defaultValue: '',
  );

  /// WebSocket endpoint relative path (default: `/ws`).
  /// Sourced strictly at compile-time from `--dart-define=WS_ENDPOINT=...`
  static const String wsPath = String.fromEnvironment(
    'WS_ENDPOINT',
    defaultValue: '/ws',
  );

  /// Resolves the effective WebSocket endpoint URL.
  static String get wsBaseUrl {
    if (_rawWsBaseUrl.isNotEmpty) {
      return _rawWsBaseUrl.replaceAll(RegExp(r'/+$'), '');
    }
    final isSecure = useSecureSchemes || port == '443';
    final scheme = isSecure ? 'wss' : 'ws';
    final hasStandardPort = port.isEmpty || port == '80' || port == '443';
    final host = hasStandardPort ? appDomain : '$appDomain:$port';
    final path = wsPath.startsWith('/') ? wsPath : '/$wsPath';
    return '$scheme://$host$path';
  }

  /// Endpoint that exchanges an authenticated access token for a one-use
  /// WebSocket ticket. The long-lived bearer token never enters the WS URL.
  static const String wsTicketPath = String.fromEnvironment(
    'WS_TICKET_ENDPOINT',
    defaultValue: '/api/ws-ticket',
  );

  static String get wsTicketUrl {
    if (wsTicketPath.startsWith('http://') || wsTicketPath.startsWith('https://')) {
      return wsTicketPath;
    }
    final path = wsTicketPath.startsWith('/') ? wsTicketPath : '/$wsTicketPath';
    return '${apiBaseUrl.replaceAll(RegExp(r'/+$'), '')}$path';
  }

  /// Catch-up synchronization endpoint path (default: `/api/messages/sync`).
  /// Sourced strictly at compile-time from `--dart-define=SYNC_ENDPOINT=...`
  static const String syncPath = String.fromEnvironment(
    'SYNC_ENDPOINT',
    defaultValue: '/api/messages/sync',
  );

  /// Resolves the full URL for the catch-up synchronization REST service.
  static String syncUrl({String? baseUrlOverride}) {
    if (syncPath.startsWith('http://') || syncPath.startsWith('https://')) {
      return syncPath;
    }
    final base = (baseUrlOverride ?? apiBaseUrl).replaceAll(RegExp(r'/+$'), '');
    final path = syncPath.startsWith('/') ? syncPath : '/$syncPath';
    return '$base$path';
  }

  /// Service health-check endpoint URL (default: `$apiBaseUrl/healthz`).
  /// Sourced strictly at compile-time from `--dart-define=HEALTH_ENDPOINT=...`
  static const String healthPath = String.fromEnvironment(
    'HEALTH_ENDPOINT',
    defaultValue: '/healthz',
  );

  static String get healthCheckUrl {
    if (healthPath.startsWith('http://') || healthPath.startsWith('https://')) {
      return healthPath;
    }
    final base = apiBaseUrl.replaceAll(RegExp(r'/+$'), '');
    final path = healthPath.startsWith('/') ? healthPath : '/$healthPath';
    return '$base$path';
  }

  /// Convenience helper indicating if running under development environment.
  static bool get isDevelopment => environment == 'development';

  // ===========================================================================
  // Immich Integration Endpoints (Release 2 Preparation)
  // ===========================================================================

  /// Immich instance domain name (e.g. 'immich.example.home.arpa').
  /// Sourced strictly at compile-time from `--dart-define=IMMICH_DOMAIN=...`
  static const String immichDomain = String.fromEnvironment(
    'IMMICH_DOMAIN',
    defaultValue: '',
  );

  /// Immich REST API base URL (e.g. 'https://immich.example.home.arpa/api').
  /// Sourced strictly at compile-time from `--dart-define=IMMICH_API_URL=...`
  static const String _rawImmichApiUrl = String.fromEnvironment(
    'IMMICH_API_URL',
    defaultValue: '',
  );
  static String get immichApiUrl {
    if (_rawImmichApiUrl.isNotEmpty) {
      return _rawImmichApiUrl.replaceAll(RegExp(r'/+$'), '');
    }
    if (immichDomain.isEmpty) return '';
    final isSecure = useSecureSchemes || port == '443';
    final scheme = isSecure ? 'https' : 'http';
    return '$scheme://$immichDomain/api';
  }

  // NOTE: there is intentionally no IMMICH_API_KEY here. Anything passed with
  // --dart-define is compiled into every binary and the web bundle; the Immich
  // key must stay on the server, which proxies Immich requests (Release 2).
}
