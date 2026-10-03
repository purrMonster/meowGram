/// Runtime and compile-time configuration contract for meowGram client.
///
/// All values are sourced strictly via `--dart-define` compile-time variables
/// or runtime fallback configuration to ensure zero hardcoded domains.
class AppConfig {
  const AppConfig._();

  /// Canonical application name
  static const String appName = 'meowGram';

  /// Current environment name: development, staging, production
  static const String environment = String.fromEnvironment(
    'APP_ENV',
    defaultValue: 'development',
  );

  /// Target application domain (e.g. localhost, meowgram.example.com)
  static const String appDomain = String.fromEnvironment(
    'APP_DOMAIN',
    defaultValue: 'localhost',
  );

  /// Target HTTP port (e.g. 8080)
  static const String port = String.fromEnvironment(
    'HTTP_PORT',
    defaultValue: '8080',
  );

  /// Whether to enforce TLS/WSS secure protocols
  static const bool useSecureSchemes = bool.fromEnvironment(
    'USE_SECURE_SCHEMES',
    defaultValue: false,
  );

  /// Optional direct override for API base URL:
  /// e.g. `--dart-define=API_BASE_URL=https://api.meowgram.com`
  static const String _apiBaseUrlOverride = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: '',
  );

  /// Optional direct override for WebSocket base URL:
  /// e.g. `--dart-define=WS_BASE_URL=wss://api.meowgram.com/ws`
  static const String _wsBaseUrlOverride = String.fromEnvironment(
    'WS_BASE_URL',
    defaultValue: '',
  );

  /// Resolves the effective HTTP Base URL
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

  /// Resolves the effective WebSocket endpoint URL
  static String get wsBaseUrl {
    if (_wsBaseUrlOverride.isNotEmpty) {
      return _wsBaseUrlOverride;
    }
    final scheme = useSecureSchemes ? 'wss' : 'ws';
    final hasStandardPort = port.isEmpty || port == '80' || port == '443';
    final host = hasStandardPort ? appDomain : '$appDomain:$port';
    return '$scheme://$host/ws';
  }

  /// Health-check endpoint URL
  static String get healthCheckUrl => '$apiBaseUrl/healthz';

  /// Whether running in development mode
  static bool get isDevelopment => environment == 'development';
}
