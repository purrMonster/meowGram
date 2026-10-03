import 'package:flutter/foundation.dart';
import 'package:meowgram_client/src/config/env_loader.dart';

/// Runtime and compile-time configuration contract for the meowGram client.
///
/// In alignment with the "Zero Hardcoded Domains" architectural mandate,
/// all service targets, authentication parameters, endpoints, and domain references
/// are controllable strictly via environment variables, `.env` configuration files,
/// or compile-time `--dart-define` / `--dart-define-from-file` parameters.
class AppConfig {
  const AppConfig._();

  /// In-memory runtime environment overrides loaded from local `.env` files or dynamic tests.
  static final Map<String, String> _runtimeOverrides = <String, String>{};

  /// Initializes configuration by scanning candidate `.env` files and process environment.
  static Future<void> initialize() async {
    try {
      final loaded = await EnvLoader.load();
      if (loaded.isNotEmpty) {
        _runtimeOverrides.addAll(loaded);
      }
    } catch (e) {
      debugPrint('AppConfig initialization error: $e');
    }
  }

  /// Parses and applies key-value pairs from a `.env` format string.
  static void loadFromEnvString(String content) {
    final lines = content.split('\n');
    for (var line in lines) {
      line = line.trim();
      if (line.isEmpty || line.startsWith('#')) continue;
      final eqIdx = line.indexOf('=');
      if (eqIdx <= 0) continue;
      final key = line.substring(0, eqIdx).trim();
      var val = line.substring(eqIdx + 1).trim();

      if ((val.startsWith('"') && val.endsWith('"')) ||
          (val.startsWith("'") && val.endsWith("'"))) {
        if (val.length >= 2) {
          val = val.substring(1, val.length - 1);
        }
      }
      if (key.isNotEmpty) {
        _runtimeOverrides[key] = val;
      }
    }
  }

  /// Explicitly injects runtime overrides (useful for unit tests and profile switching).
  static void setOverrides(Map<String, String> overrides) {
    _runtimeOverrides.addAll(overrides);
  }

  /// Clears all runtime overrides, reverting to `--dart-define` and default values.
  static void clearOverrides() {
    _runtimeOverrides.clear();
  }

  /// Sourcing helper: precedence is runtime `.env` override -> compile-time `--dart-define` -> default value.
  static String _get(String key, {String defaultValue = ''}) {
    final overrideVal = _runtimeOverrides[key];
    if (overrideVal != null && overrideVal.trim().isNotEmpty) {
      return overrideVal.trim();
    }
    return String.fromEnvironment(key, defaultValue: defaultValue);
  }

  /// Boolean sourcing helper.
  static bool _getBool(String key, {bool defaultValue = false}) {
    final overrideVal = _runtimeOverrides[key];
    if (overrideVal != null && overrideVal.trim().isNotEmpty) {
      final normalized = overrideVal.trim().toLowerCase();
      return normalized == 'true' || normalized == '1' || normalized == 'yes';
    }
    return bool.fromEnvironment(key, defaultValue: defaultValue);
  }

  // ===========================================================================
  // Application & Core Host Routing
  // ===========================================================================

  /// Canonical application brand name displayed in the UI.
  static String get appName => _get('APP_NAME', defaultValue: 'meowGram');

  /// Environment mode: 'development', 'staging', or 'production'.
  static String get environment => _get(
        'APP_ENV',
        defaultValue: _get('ENVIRONMENT', defaultValue: 'development'),
      );

  /// Target application domain (e.g., 'localhost', 'meowgram.local', 'meowgram.purrbrews.cc').
  static String get appDomain => _get('APP_DOMAIN', defaultValue: 'localhost');

  /// Target HTTP port for backend services (default: 8080).
  static String get port => _get('HTTP_PORT', defaultValue: _get('PORT', defaultValue: '8080'));

  /// Whether to enforce secure protocols (https:// and wss://) instead of http/ws.
  static bool get useSecureSchemes => _getBool('USE_SECURE_SCHEMES', defaultValue: false);

  // ===========================================================================
  // Authelia OpenID Connect (OIDC) PKCE Configuration & Endpoints
  // ===========================================================================

  /// Authelia Domain (e.g., 'auth.purrbrews.cc', 'localhost:9091', 'auth.example.com').
  static String get autheliaDomain {
    final explicitDomain = _get('AUTHELIA_DOMAIN');
    if (explicitDomain.isNotEmpty) {
      return explicitDomain;
    }
    // If AUTHELIA_ISSUER_URL or AUTHELIA_ISSUER is set, parse host from it
    final issuer = _get('AUTHELIA_ISSUER_URL', defaultValue: _get('AUTHELIA_ISSUER'));
    if (issuer.isNotEmpty) {
      final uri = Uri.tryParse(issuer);
      if (uri != null && uri.host.isNotEmpty) {
        return uri.hasPort ? '${uri.host}:${uri.port}' : uri.host;
      }
    }
    return 'localhost:9091';
  }

  /// Client ID registered in Authelia's identity provider configuration.
  static String get autheliaClientId => _get('AUTHELIA_CLIENT_ID', defaultValue: 'meowgram-client');

  /// Base issuer URL for Authelia's OpenID Connect provider.
  static String get autheliaIssuerUrl {
    // 1. Direct explicit issuer URL takes top precedence
    final explicitIssuer = _get('AUTHELIA_ISSUER_URL', defaultValue: _get('AUTHELIA_ISSUER'));
    if (explicitIssuer.isNotEmpty) {
      return explicitIssuer.replaceAll(RegExp(r'/+$'), '');
    }

    // 2. Automatically derive from AUTHELIA_DOMAIN
    final domain = autheliaDomain;
    if (domain.startsWith('http://') || domain.startsWith('https://')) {
      return domain.replaceAll(RegExp(r'/+$'), '');
    }

    final isLocal = domain.contains('localhost') || domain.startsWith('127.');
    final scheme = (useSecureSchemes || !isLocal) ? 'https' : 'http';
    return '$scheme://$domain';
  }

  /// JSON Web Key Set (JWKS) URL used for signature validation.
  static String get autheliaJwksUrl => _get(
        'AUTHELIA_JWKS_URL',
        defaultValue: '$autheliaIssuerUrl/jwks.json',
      );

  /// OpenID Discovery configuration endpoint URL.
  static String get autheliaDiscoveryUrl => _get(
        'AUTHELIA_DISCOVERY_URL',
        defaultValue: '$autheliaIssuerUrl/.well-known/openid-configuration',
      );

  /// Authelia OIDC Authorization endpoint.
  static String get autheliaAuthorizationEndpoint => _get(
        'AUTHELIA_AUTHORIZATION_ENDPOINT',
        defaultValue: '$autheliaIssuerUrl/api/oidc/authorization',
      );

  /// Authelia OIDC Token exchange endpoint.
  static String get autheliaTokenEndpoint => _get(
        'AUTHELIA_TOKEN_ENDPOINT',
        defaultValue: '$autheliaIssuerUrl/api/oidc/token',
      );

  /// Authelia OIDC UserInfo endpoint.
  static String get autheliaUserinfoEndpoint => _get(
        'AUTHELIA_USERINFO_ENDPOINT',
        defaultValue: '$autheliaIssuerUrl/api/oidc/userinfo',
      );

  /// Authelia OIDC Token revocation endpoint.
  static String get autheliaRevocationEndpoint => _get(
        'AUTHELIA_REVOCATION_ENDPOINT',
        defaultValue: '$autheliaIssuerUrl/api/oidc/revocation',
      );

  /// Resolved OAuth 2.0 / OIDC redirect URI based on platform context.
  static String get authRedirectUri {
    final overrideUri = _get('AUTH_REDIRECT_URI');
    if (overrideUri.isNotEmpty) {
      return overrideUri;
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
  static List<String> get oidcScopes {
    final customScopes = _get('OIDC_SCOPES');
    if (customScopes.isNotEmpty) {
      return customScopes
          .split(RegExp(r'[, ]+'))
          .where((s) => s.isNotEmpty)
          .toList();
    }
    return const <String>[
      'openid',
      'profile',
      'email',
      'offline_access',
    ];
  }

  // ===========================================================================
  // Backend Service Endpoint Resolvers
  // ===========================================================================

  /// Resolves the effective HTTP Base URL.
  static String get apiBaseUrl {
    final overrideUrl = _get('API_BASE_URL');
    if (overrideUrl.isNotEmpty) {
      return overrideUrl.replaceAll(RegExp(r'/+$'), '');
    }
    final scheme = useSecureSchemes ? 'https' : 'http';
    final hasStandardPort = port.isEmpty || port == '80' || port == '443';
    return hasStandardPort
        ? '$scheme://$appDomain'
        : '$scheme://$appDomain:$port';
  }

  /// WebSocket endpoint relative path (default: `/ws`).
  static String get wsPath => _get(
        'WS_ENDPOINT',
        defaultValue: _get('WS_PATH', defaultValue: '/ws'),
      );

  /// Resolves the effective WebSocket endpoint URL.
  static String get wsBaseUrl {
    final overrideUrl = _get('WS_BASE_URL');
    if (overrideUrl.isNotEmpty) {
      return overrideUrl.replaceAll(RegExp(r'/+$'), '');
    }
    final scheme = useSecureSchemes ? 'wss' : 'ws';
    final hasStandardPort = port.isEmpty || port == '80' || port == '443';
    final host = hasStandardPort ? appDomain : '$appDomain:$port';
    final path = wsPath.startsWith('/') ? wsPath : '/$wsPath';
    return '$scheme://$host$path';
  }

  /// Builds a WebSocket URL with an authenticated Bearer token query parameter.
  static String authenticatedWsUrl(String accessToken) {
    final base = wsBaseUrl;
    final separator = base.contains('?') ? '&' : '?';
    return '$base${separator}token=${Uri.encodeComponent(accessToken)}';
  }

  /// Catch-up synchronization endpoint path or relative route (default: `/api/messages/sync`).
  static String get syncPath => _get(
        'SYNC_ENDPOINT',
        defaultValue: _get('MESSAGES_SYNC_ENDPOINT', defaultValue: '/api/messages/sync'),
      );

  /// Resolves the full URL for the catch-up synchronization REST service.
  static String syncUrl({String? baseUrlOverride}) {
    final explicitSync = _get('SYNC_ENDPOINT', defaultValue: _get('MESSAGES_SYNC_ENDPOINT'));
    if (explicitSync.startsWith('http://') || explicitSync.startsWith('https://')) {
      return explicitSync;
    }
    final base = (baseUrlOverride ?? apiBaseUrl).replaceAll(RegExp(r'/+$'), '');
    final path = syncPath.startsWith('/') ? syncPath : '/$syncPath';
    return '$base$path';
  }

  /// Service health-check endpoint URL (default: `$apiBaseUrl/healthz`).
  static String get healthCheckUrl {
    final explicitHealth = _get('HEALTH_ENDPOINT');
    if (explicitHealth.startsWith('http://') || explicitHealth.startsWith('https://')) {
      return explicitHealth;
    }
    final base = apiBaseUrl.replaceAll(RegExp(r'/+$'), '');
    final path = explicitHealth.isNotEmpty ? explicitHealth : '/healthz';
    return '$base${path.startsWith('/') ? path : '/$path'}';
  }

  /// Convenience helper indicating if running under development environment.
  static bool get isDevelopment => environment == 'development';

  // ===========================================================================
  // Immich Integration Endpoints (Release 2 Preparation)
  // ===========================================================================

  /// Immich instance domain name (e.g. 'immich.purrbrews.cc').
  static String get immichDomain => _get('IMMICH_DOMAIN', defaultValue: 'immich.purrbrews.cc');

  /// Immich REST API base URL (e.g. 'https://immich.purrbrews.cc/api').
  static String get immichApiUrl {
    final explicitApi = _get('IMMICH_API_URL');
    if (explicitApi.isNotEmpty) {
      return explicitApi.replaceAll(RegExp(r'/+$'), '');
    }
    final scheme = useSecureSchemes ? 'https' : 'http';
    return '$scheme://$immichDomain/api';
  }

  /// Immich API Key used for server-side proxy authentication.
  static String get immichApiKey => _get('IMMICH_API_KEY');
}
